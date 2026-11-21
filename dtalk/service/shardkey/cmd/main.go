package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/web3chat/txchat/dtalk/pkg/redis"
	"github.com/web3chat/txchat/dtalk/service/shardkey/config"
	"github.com/web3chat/txchat/dtalk/service/shardkey/internal/handler"
	"github.com/web3chat/txchat/dtalk/service/shardkey/internal/service"
	"github.com/web3chat/txchat/dtalk/service/shardkey/internal/store"
)

const srvName = "shardkey"

var (
	// 由构建脚本通过 -ldflags -X main.xxx 注入
	projectVersion = "1.0.0"
	goVersion      = ""
	gitCommit      = ""
	buildTime      = ""
	osArch         = ""

	confPath      = flag.String("conf", "config/shardkey.toml", "config file path")
	isShowVersion = flag.Bool("v", false, "show version")
)

func main() {
	flag.Parse()
	if *isShowVersion {
		showVersion()
		return
	}

	cfg, err := config.Load(*confPath)
	if err != nil {
		// 配置错误直接退出，避免带着半套配置启动
		logrus.WithError(err).Fatal("load config failed")
	}
	initLogger()

	db, err := store.InitDB(cfg)
	if err != nil {
		logrus.WithError(err).Fatal("init mysql failed")
	}
	defer db.Close()

	redisPool := redis.New(cfg.Redis)
	if _, err := redisPool.Do("PING"); err != nil {
		// Redis 在 im compose 中启动，dtalk 单独重启时可能还没就绪；这里只告警，连接池会自动重连。
		logrus.WithError(err).Warn("redis is not ready, verification code requests may fail")
	}
	verificationStore := service.NewRedisVerificationStore(redisPool)
	smsService := service.NewSMSService(cfg, verificationStore)
	shardStore := store.NewShardStore(db, cfg)
	bindingStore := store.NewBindingStore(db)

	r := gin.New()
	r.Use(gin.LoggerWithFormatter(ginLogFormatter))
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	handler.RegisterRoutes(r, cfg, smsService, shardStore, bindingStore)

	srv := &http.Server{
		Addr:    cfg.HttpServer.Addr,
		Handler: r,
	}

	go func() {
		logrus.WithFields(logrus.Fields{
			"addr":    cfg.HttpServer.Addr,
			"version": projectVersion,
		}).Info("shardkey server starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Fatal("server stopped unexpectedly")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logrus.WithError(err).Error("server shutdown failed")
	}
	logrus.Info("shardkey server exited")
}

func initLogger() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)
}

func showVersion() {
	fmt.Printf("Project: %s\n", srvName)
	fmt.Printf(" Version: %s\n", projectVersion)
	fmt.Printf(" Go Version: %s\n", goVersion)
	fmt.Printf(" Git Commit: %s\n", gitCommit)
	fmt.Printf(" Built: %s\n", buildTime)
	fmt.Printf(" OS/Arch: %s\n", osArch)
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 服务只供 App / 内部调用，默认不放开通配 CORS。
		// 需要浏览器调试时在 nginx 层按域名放行。
		c.Next()
	}
}

func ginLogFormatter(param gin.LogFormatterParams) string {
	return fmt.Sprintf("%s - [%s] \"%s %s %s %d\" %s\n",
		param.ClientIP,
		param.TimeStamp.Format(time.RFC3339),
		param.Method,
		param.Path,
		param.Request.Proto,
		param.StatusCode,
		param.Latency,
	)
}
