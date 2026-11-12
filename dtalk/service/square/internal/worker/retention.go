package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/usecase"
)

func StartRetention(ctx context.Context, svc *usecase.Service) {
	if svc == nil {
		return
	}
	cfg := svc.Config()
	if cfg.Retention.TTLDays <= 0 {
		slog.Info("retention disabled", "ttl_days", 0)
		return
	}
	interval := cfg.Retention.Interval
	if interval <= 0 {
		interval = time.Hour
	}
	slog.Info("retention started", "ttl_days", cfg.Retention.TTLDays, "interval", interval.String(), "oss_delete", cfg.OSS.DeleteEnabled)
	go runRetention(ctx, svc, interval)
}

func runRetention(ctx context.Context, svc *usecase.Service, interval time.Duration) {
	sweep(ctx, svc)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("retention stopped")
			return
		case <-t.C:
			sweep(ctx, svc)
		}
	}
}

func sweep(ctx context.Context, svc *usecase.Service) {
	n, err := svc.SweepExpiredPosts(ctx)
	if err != nil {
		slog.Error("retention sweep failed", "err", err)
		return
	}
	if n > 0 {
		slog.Info("retention sweep ok", "deleted", n)
	}
}
