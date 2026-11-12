package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/web3chat/txchat/dtalk/service/chatclean/dao"
)

const (
	defaultKeep               = 168 * time.Hour
	defaultBatch              = 500
	defaultMaxBatches         = 200
	defaultMaxObjects         = 2000
	groupMidBatch             = 100
	redisKeyCap               = 500
	auditSourceRetention int8 = 2
	retentionOperator         = "retention"
	retentionTarget           = "global"
)

// retentionPause 是批与批之间的停顿。测试里可以改成 0。
var retentionPause = 10 * time.Millisecond

// RetentionReport 是一轮全局保留的结果。Skipped 表示配置关闭，没有删数据。
type RetentionReport struct {
	Skipped          bool
	CutoffMs         int64
	CutoffMid        int64
	PrivateDeleted   int64
	GroupDeleted     int64
	PrivateBatches   int
	GroupBatches     int
	PrivateMore      bool
	GroupMore        bool
	RedisRemoved     int
	RedisMore        bool
	OSSRemoved       int
	OSSSkippedSquare int
	OSSMore          bool
}

type retentionSettings struct {
	Enable     bool
	Keep       time.Duration
	MaxBatches int
	BatchSize  int
	MaxObjects int
}

type expiredStore interface {
	DeleteExpiredContent(ctx context.Context, contentTable, relationTable string, cutoffMid, cutoffMs int64, limit int) (int, error)
	DeleteExpiredRelations(ctx context.Context, relationTable string, cutoffMid, cutoffMs int64, limit int) (int64, error)
}

type channelResult struct {
	Deleted int64
	Batches int
	More    bool
}

// RunGlobalRetention 静默删除早于 now-Keep 的私聊和群聊。
// 不写撤回信令。MinIO 或 Redis 失败只记日志，不回滚已经删掉的库记录。
func (s *Service) RunGlobalRetention(ctx context.Context) (RetentionReport, error) {
	cfg := s.retentionSettings()
	if !cfg.Enable {
		return RetentionReport{Skipped: true}, nil
	}

	now := time.Now()
	cutoffMs, cutoffMid := Cutoff(now, cfg.Keep)
	report := RetentionReport{CutoffMs: cutoffMs, CutoffMid: cutoffMid}

	private, err := s.purgeLogged(ctx, dao.PrivateContentTable, dao.PrivateRelationTable, cutoffMid, cutoffMs, cfg.BatchSize, cfg.MaxBatches)
	report.PrivateDeleted = private.Deleted
	report.PrivateBatches = private.Batches
	report.PrivateMore = private.More
	if err != nil {
		s.writeRetentionAudit(ctx, report)
		return report, err
	}

	group, err := s.purgeLogged(ctx, dao.GroupContentTable, dao.GroupRelationTable, cutoffMid, cutoffMs, channelBatch(cfg.BatchSize, groupMidBatch), cfg.MaxBatches)
	report.GroupDeleted = group.Deleted
	report.GroupBatches = group.Batches
	report.GroupMore = group.More
	if err != nil {
		s.writeRetentionAudit(ctx, report)
		return report, err
	}

	s.trimRedis(ctx, cutoffMs, &report)
	s.cleanConfiguredOSS(ctx, now, cfg, &report)
	s.writeRetentionAudit(ctx, report)
	return report, nil
}

func (s *Service) purgeLogged(ctx context.Context, contentTable, relationTable string, cutoffMid, cutoffMs int64, batch, maxBatches int) (channelResult, error) {
	res, err := purgeChannel(ctx, s.dao, contentTable, relationTable, cutoffMid, cutoffMs, batch, maxBatches)
	if err != nil {
		log.Error().Err(err).
			Str("table", contentTable).
			Str("relation", relationTable).
			Int64("cutoff_ms", cutoffMs).
			Int64("cutoff_mid", cutoffMid).
			Int("batch", batch).
			Int("batches", res.Batches).
			Int("max_batches", maxBatches).
			Int64("deleted", res.Deleted).
			Bool("more", true).
			Msg("retention delete failed")
		return res, err
	}
	if res.Deleted > 0 || res.More {
		log.Info().
			Str("table", contentTable).
			Str("relation", relationTable).
			Int64("cutoff_ms", cutoffMs).
			Int64("cutoff_mid", cutoffMid).
			Int("batch", batch).
			Int("batches", res.Batches).
			Int("max_batches", maxBatches).
			Int64("deleted", res.Deleted).
			Bool("more", res.More).
			Msg("retention channel")
	}
	return res, nil
}

func purgeChannel(ctx context.Context, store expiredStore, contentTable, relationTable string, cutoffMid, cutoffMs int64, batch, maxBatches int) (channelResult, error) {
	var res channelResult
	if batch <= 0 || maxBatches <= 0 {
		return res, fmt.Errorf("retention batch=%d maxBatches=%d table=%s", batch, maxBatches, contentTable)
	}

	for {
		if err := ctx.Err(); err != nil {
			res.More = true
			return res, err
		}
		n, err := store.DeleteExpiredContent(ctx, contentTable, relationTable, cutoffMid, cutoffMs, batch)
		if err != nil {
			res.More = true
			return res, err
		}
		if n == 0 {
			break
		}
		res.Batches++
		res.Deleted += int64(n)
		if n < batch || res.Batches >= maxBatches {
			if n == batch && res.Batches >= maxBatches {
				res.More = true
			}
			if res.More {
				return res, nil
			}
			break
		}
		sleepRetention()
	}

	for res.Batches < maxBatches {
		if err := ctx.Err(); err != nil {
			res.More = true
			return res, err
		}
		n, err := store.DeleteExpiredRelations(ctx, relationTable, cutoffMid, cutoffMs, batch)
		if err != nil {
			res.More = true
			return res, err
		}
		if n == 0 {
			return res, nil
		}
		res.Batches++
		res.Deleted += n
		if n < int64(batch) {
			return res, nil
		}
		if res.Batches >= maxBatches {
			res.More = true
			return res, nil
		}
		sleepRetention()
	}
	return res, nil
}

func (s *Service) trimRedis(ctx context.Context, cutoffMs int64, report *RetentionReport) {
	if s.cfg == nil || !s.cfg.Redis.Enable {
		return
	}
	if !s.dao.HasRedis() {
		log.Error().Int64("cutoff_ms", cutoffMs).Msg("retention redis enabled but addr empty, skip")
		return
	}
	removed, more, err := s.dao.TrimRecordCache(ctx, uint64(cutoffMs), redisKeyCap)
	report.RedisRemoved = removed
	report.RedisMore = more
	if err != nil {
		log.Error().Err(err).
			Int64("cutoff_ms", cutoffMs).
			Int("removed", removed).
			Bool("more", more).
			Msg("retention redis trim failed")
		return
	}
	if removed > 0 || more {
		log.Info().
			Int64("cutoff_ms", cutoffMs).
			Int("removed", removed).
			Bool("more", more).
			Msg("retention redis trim")
	}
}

func (s *Service) cleanConfiguredOSS(ctx context.Context, now time.Time, cfg retentionSettings, report *RetentionReport) {
	if s.cfg == nil || !s.cfg.MinIO.Enable {
		return
	}
	if s.cfg.MinIO.Endpoint == "" || s.cfg.MinIO.Bucket == "" {
		log.Error().Int64("cutoff_ms", report.CutoffMs).Msg("retention minio enabled but endpoint or bucket empty, skip")
		return
	}
	store, err := newMinioStore(s.cfg.MinIO)
	if err != nil {
		log.Error().Err(err).Str("endpoint", s.cfg.MinIO.Endpoint).Str("bucket", s.cfg.MinIO.Bucket).Msg("retention minio client")
		return
	}
	day := CutoffDay(now, cfg.Keep)
	oss, err := cleanExpiredObjects(ctx, store, mediaPrefixes, day, cfg.MaxObjects)
	report.OSSRemoved = oss.Removed
	report.OSSSkippedSquare = oss.SkippedSquare
	report.OSSMore = oss.More
	if err != nil {
		log.Error().Err(err).
			Str("cutoff_day", day).
			Int("removed", oss.Removed).
			Int("skipped_square", oss.SkippedSquare).
			Bool("more", oss.More).
			Msg("retention oss failed")
	}
}

func (s *Service) writeRetentionAudit(ctx context.Context, report RetentionReport) {
	total := report.PrivateDeleted + report.GroupDeleted
	if total == 0 && !report.PrivateMore && !report.GroupMore {
		return
	}
	err := s.dao.WriteAuditLog(ctx, retentionOperator, retentionTarget, auditSourceRetention, 0, 0, report.CutoffMs, total, time.Now().UnixMilli())
	if err != nil {
		log.Error().Err(err).
			Int64("cutoff_ms", report.CutoffMs).
			Int64("deleted", total).
			Bool("private_more", report.PrivateMore).
			Bool("group_more", report.GroupMore).
			Msg("retention audit failed")
	}
}

func (s *Service) retentionSettings() retentionSettings {
	cfg := retentionSettings{
		Enable:     true,
		Keep:       defaultKeep,
		MaxBatches: defaultMaxBatches,
		BatchSize:  defaultBatch,
		MaxObjects: defaultMaxObjects,
	}
	if s.cfg == nil {
		return cfg
	}
	cfg.Enable = s.cfg.Retention.Enable
	if s.cfg.Retention.Keep > 0 {
		cfg.Keep = time.Duration(s.cfg.Retention.Keep)
	}
	if s.cfg.Retention.MaxBatches > 0 {
		cfg.MaxBatches = s.cfg.Retention.MaxBatches
	}
	if s.cfg.Scheduler != nil && s.cfg.Scheduler.BatchSize > 0 {
		cfg.BatchSize = s.cfg.Scheduler.BatchSize
	}
	if s.cfg.MinIO.MaxObjects > 0 {
		cfg.MaxObjects = s.cfg.MinIO.MaxObjects
	}
	return cfg
}

func channelBatch(batch, cap int) int {
	if batch <= 0 {
		batch = defaultBatch
	}
	if cap > 0 && batch > cap {
		return cap
	}
	return batch
}

func sleepRetention() {
	if retentionPause > 0 {
		time.Sleep(retentionPause)
	}
}
