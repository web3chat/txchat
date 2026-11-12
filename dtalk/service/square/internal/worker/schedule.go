package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/usecase"
)

const officialScheduleEvery = time.Minute

// StartOfficialSchedule 每分钟发布到点的官方帖，并软删到点该自动删除的官方帖。
func StartOfficialSchedule(ctx context.Context, svc *usecase.Service) {
	if svc == nil {
		return
	}
	slog.Info("official schedule started", "interval", officialScheduleEvery.String())
	go runOfficialSchedule(ctx, svc)
}

func runOfficialSchedule(ctx context.Context, svc *usecase.Service) {
	sweepOfficialSchedule(ctx, svc)
	t := time.NewTicker(officialScheduleEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("official schedule stopped")
			return
		case <-t.C:
			sweepOfficialSchedule(ctx, svc)
		}
	}
}

func sweepOfficialSchedule(ctx context.Context, svc *usecase.Service) {
	published, expired, err := svc.SweepOfficialSchedule(ctx)
	if err != nil {
		slog.Error("official schedule sweep failed", "err", err)
		return
	}
	if published > 0 || expired > 0 {
		slog.Info("official schedule sweep ok", "published", published, "expired", expired)
	}
}
