package scheduler

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/web3chat/txchat/dtalk/service/chatclean/config"
	"github.com/web3chat/txchat/dtalk/service/chatclean/dao"
	"github.com/web3chat/txchat/dtalk/service/chatclean/service"
)

// Scheduler 先按会话自动删除，再执行全局 168 小时保留。
type Scheduler struct {
	svc      *service.Service
	interval time.Duration
	quit     chan struct{}
}

func New(cfg *config.Config, svc *service.Service) *Scheduler {
	return &Scheduler{
		svc:      svc,
		interval: time.Duration(cfg.Scheduler.Interval),
		quit:     make(chan struct{}),
	}
}

// Start 启动调度器（非阻塞）。
func (s *Scheduler) Start() {
	go s.run()
}

// Stop 停止调度器。
func (s *Scheduler) Stop() {
	close(s.quit)
}

func (s *Scheduler) run() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	log.Info().Dur("interval", s.interval).Msg("chatclean scheduler started")
	for {
		select {
		case <-ticker.C:
			s.doClean()
		case <-s.quit:
			log.Info().Msg("chatclean scheduler stopped")
			return
		}
	}
}

func (s *Scheduler) doClean() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	s.runAutoDelete(ctx)
	s.runRetention(ctx)
}

func (s *Scheduler) runAutoDelete(ctx context.Context) {
	cfgs, err := s.svc.Dao().ListNonNullAutoDeleteConfigs(ctx)
	if err != nil {
		log.Error().Err(err).Msg("scheduler: ListNonNullAutoDeleteConfigs")
		return
	}
	log.Info().Int("count", len(cfgs)).Msg("scheduler: auto-delete round start")
	for _, cfg := range cfgs {
		s.processOne(ctx, cfg)
	}
	log.Info().Msg("scheduler: auto-delete round done")
}

func (s *Scheduler) runRetention(ctx context.Context) {
	report, err := s.svc.RunGlobalRetention(ctx)
	if err != nil {
		log.Error().Err(err).
			Int64("cutoff_ms", report.CutoffMs).
			Int64("private_deleted", report.PrivateDeleted).
			Int64("group_deleted", report.GroupDeleted).
			Int("private_batches", report.PrivateBatches).
			Int("group_batches", report.GroupBatches).
			Bool("private_more", report.PrivateMore).
			Bool("group_more", report.GroupMore).
			Msg("scheduler: global retention failed")
		return
	}
	if report.Skipped {
		return
	}
	log.Info().
		Int64("cutoff_ms", report.CutoffMs).
		Int64("private_deleted", report.PrivateDeleted).
		Int64("group_deleted", report.GroupDeleted).
		Int("private_batches", report.PrivateBatches).
		Int("group_batches", report.GroupBatches).
		Bool("private_more", report.PrivateMore).
		Bool("group_more", report.GroupMore).
		Int("redis_removed", report.RedisRemoved).
		Bool("redis_more", report.RedisMore).
		Int("oss_removed", report.OSSRemoved).
		Int("oss_skipped_square", report.OSSSkippedSquare).
		Bool("oss_more", report.OSSMore).
		Msg("scheduler: global retention done")
}

func (s *Scheduler) processOne(ctx context.Context, cfg *dao.AutoDeleteConfig) {
	deleted, err := s.svc.RunAutoDeleteOnce(ctx, cfg)
	if err != nil {
		log.Error().Err(err).
			Str("uid_a", cfg.UidA).
			Str("uid_b", cfg.UidB).
			Msg("scheduler: RunAutoDeleteOnce")
		return
	}
	if deleted > 0 {
		log.Info().
			Str("uid_a", cfg.UidA).
			Str("uid_b", cfg.UidB).
			Int64("deleted", deleted).
			Msg("scheduler: auto-deleted messages")
	}
}
