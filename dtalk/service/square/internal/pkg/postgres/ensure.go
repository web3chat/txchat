package postgres

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/config"
)

const (
	autoContainer = "txchat-square-pg"
	autoImage     = "postgres:16-alpine"
	autoVolume    = "txchat-square-pg-data"
	readyWait     = 60 * time.Second
	readyEvery    = time.Second
)

// EnsureAuto 在未指定 SQUARE_PG_HOST 时拉取镜像并创建/复用本地 PostgreSQL 容器。
func EnsureAuto(ctx context.Context, pg config.PostgresConfig) error {
	if !pg.Auto {
		return nil
	}
	if _, err := exec.LookPath("docker"); err != nil {
		slog.Error("postgres auto requires docker", "err", err)
		return fmt.Errorf("未指定 SQUARE_PG_HOST，且未找到 docker，无法自动创建 PostgreSQL 容器: %w", err)
	}
	if err := ensureImage(ctx); err != nil {
		return err
	}
	if err := ensureVolume(ctx); err != nil {
		return err
	}
	state, err := containerState(ctx)
	if err != nil {
		return err
	}
	switch state {
	case "running":
		slog.Info("reuse postgres container", "container", autoContainer, "port", pg.Port, "user", pg.User, "db", pg.DBName)
	case "exited":
		slog.Info("start existing postgres container", "container", autoContainer, "user", pg.User, "db", pg.DBName)
		if err := runDocker(ctx, "start", autoContainer); err != nil {
			return fmt.Errorf("start postgres container %s: %w", autoContainer, err)
		}
	default:
		slog.Info("create postgres container",
			"container", autoContainer, "image", autoImage, "port", pg.Port,
			"user", pg.User, "password", pg.Pwd, "db", pg.DBName)
		if err := createContainer(ctx, pg); err != nil {
			return err
		}
	}
	if err := waitReady(ctx, pg); err != nil {
		slog.Error("postgres container not ready", "container", autoContainer, "host", pg.Host, "port", pg.Port, "db", pg.DBName, "err", err)
		return err
	}
	slog.Info("postgres container ready", "container", autoContainer, "host", pg.Host, "port", pg.Port, "user", pg.User, "password", pg.Pwd, "db", pg.DBName)
	return nil
}

func ensureImage(ctx context.Context) error {
	if err := runDocker(ctx, "image", "inspect", autoImage); err == nil {
		return nil
	}
	slog.Info("pull postgres image", "image", autoImage)
	if err := runDocker(ctx, "pull", autoImage); err != nil {
		slog.Error("pull postgres image failed", "image", autoImage, "err", err)
		return fmt.Errorf("docker pull %s: %w", autoImage, err)
	}
	return nil
}

func ensureVolume(ctx context.Context) error {
	if err := runDocker(ctx, "volume", "inspect", autoVolume); err == nil {
		return nil
	}
	if err := runDocker(ctx, "volume", "create", autoVolume); err != nil {
		slog.Error("create postgres volume failed", "volume", autoVolume, "err", err)
		return fmt.Errorf("docker volume create %s: %w", autoVolume, err)
	}
	return nil
}

func containerState(ctx context.Context) (string, error) {
	out, err := dockerOutput(ctx, "inspect", "-f", "{{.State.Status}}", autoContainer)
	if err != nil {
		if strings.Contains(err.Error(), "No such object") || strings.Contains(strings.ToLower(out+" "+err.Error()), "no such") {
			return "missing", nil
		}
		// inspect 失败且容器不存在时 docker 退出码非 0
		low := strings.ToLower(err.Error() + out)
		if strings.Contains(low, "no such") {
			return "missing", nil
		}
		return "", fmt.Errorf("docker inspect %s: %w", autoContainer, err)
	}
	st := strings.TrimSpace(out)
	if st == "" {
		return "missing", nil
	}
	return st, nil
}

func createContainer(ctx context.Context, pg config.PostgresConfig) error {
	args := []string{
		"run", "-d",
		"--name", autoContainer,
		"--restart", "unless-stopped",
		"-e", "TZ=Asia/Shanghai",
		"-e", "POSTGRES_USER=" + pg.User,
		"-e", "POSTGRES_PASSWORD=" + pg.Pwd,
		"-e", "POSTGRES_DB=" + pg.DBName,
		"-p", pg.Port + ":5432",
		"-v", autoVolume + ":/var/lib/postgresql/data",
		autoImage,
	}
	if err := runDocker(ctx, args...); err != nil {
		slog.Error("docker run postgres failed", "container", autoContainer, "port", pg.Port, "err", err)
		return fmt.Errorf("docker run %s: %w", autoContainer, err)
	}
	return nil
}

func waitReady(ctx context.Context, pg config.PostgresConfig) error {
	deadline := time.Now().Add(readyWait)
	var last error
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		db, err := Open(pg.Host, pg.Port, pg.User, pg.Pwd, pg.DBName, pg.SSLMode)
		if err == nil {
			_ = db.Close()
			return nil
		}
		last = err
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(readyEvery):
		}
	}
	if last == nil {
		last = fmt.Errorf("timeout")
	}
	return fmt.Errorf("wait postgres %s:%s timeout: %w", pg.Host, pg.Port, last)
}

func runDocker(ctx context.Context, args ...string) error {
	_, err := dockerOutput(ctx, args...)
	return err
}

func dockerOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stdout.String() + stderr.String()
	if err != nil {
		return out, fmt.Errorf("%v: %s", err, strings.TrimSpace(out))
	}
	return stdout.String(), nil
}
