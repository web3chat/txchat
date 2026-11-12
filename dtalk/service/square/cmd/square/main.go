package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/config"
	"github.com/web3chat/txchat/dtalk/service/square/internal/moderator"
	"github.com/web3chat/txchat/dtalk/service/square/internal/pkg/postgres"
	"github.com/web3chat/txchat/dtalk/service/square/internal/repository"
	xhttp "github.com/web3chat/txchat/dtalk/service/square/internal/server/http"
	"github.com/web3chat/txchat/dtalk/service/square/internal/usecase"
	"github.com/web3chat/txchat/dtalk/service/square/internal/worker"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err := config.Init(); err != nil {
		slog.Error("config init failed", "err", err)
		os.Exit(1)
	}
	cfg := config.Conf
	if config.IsProd(cfg.Env) {
		slog.Info("mock auth disabled", "env", cfg.Env)
	} else {
		slog.Info("mock auth", "allowed", config.IsMockAllowed(cfg.Env, cfg.AllowMock), "env", cfg.Env)
	}
	slog.Info("oss upload", "url", cfg.OSS.UploadURL(), "appId", cfg.OSS.AppID, "privkey_set", strings.TrimSpace(cfg.OSS.PrivKey) != "")
	if config.IsProd(cfg.Env) {
		slog.Info("postgres config", "auto", cfg.Postgres.Auto, "host", cfg.Postgres.Host, "port", cfg.Postgres.Port, "user", cfg.Postgres.User, "db", cfg.Postgres.DBName, "password_set", cfg.Postgres.Pwd != "")
	} else {
		slog.Info("postgres config", "auto", cfg.Postgres.Auto, "host", cfg.Postgres.Host, "port", cfg.Postgres.Port, "user", cfg.Postgres.User, "password", cfg.Postgres.Pwd, "db", cfg.Postgres.DBName)
	}

	if cfg.Postgres.Auto {
		ensCtx, ensCancel := context.WithTimeout(context.Background(), 3*time.Minute)
		err := postgres.EnsureAuto(ensCtx, cfg.Postgres)
		ensCancel()
		if err != nil {
			slog.Error("ensure postgres failed", "host", cfg.Postgres.Host, "port", cfg.Postgres.Port, "db", cfg.Postgres.DBName, "err", err)
			os.Exit(1)
		}
	}

	db, err := postgres.Open(cfg.Postgres.Host, cfg.Postgres.Port, cfg.Postgres.User, cfg.Postgres.Pwd, cfg.Postgres.DBName, cfg.Postgres.SSLMode)
	if err != nil {
		slog.Error("open postgres failed", "host", cfg.Postgres.Host, "db", cfg.Postgres.DBName, "err", err)
		os.Exit(1)
	}
	defer db.Close()

	store := repository.New(db)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := store.EnsureSchema(ctx, cfg.SchemaPath); err != nil {
		cancel()
		slog.Error("ensure schema failed", "path", cfg.SchemaPath, "err", err)
		os.Exit(1)
	}
	cancel()

	mod := moderator.NewDeepSeek(cfg.DeepSeek)
	svc := usecase.New(store, mod, cfg)
	bootCtx, bootCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := svc.BootstrapAdmin(bootCtx); err != nil {
		bootCancel()
		slog.Error("bootstrap admin failed", "err", err)
		os.Exit(1)
	}
	bootCancel()

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()
	worker.StartRetention(rootCtx, svc)
	worker.StartOfficialSchedule(rootCtx, svc)

	srv := xhttp.New(svc)
	go func() {
		slog.Info("square http listen", "addr", cfg.HTTP.Addr)
		if err := srv.ListenAndServe(); err != nil {
			slog.Error("http server exit", "err", err)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	sig := <-ch
	slog.Info("signal received", "sig", sig.String())
	rootCancel()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("http shutdown failed", "err", err)
	}
	slog.Info("square exit")
}
