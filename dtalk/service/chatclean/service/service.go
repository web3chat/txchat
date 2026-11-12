package service

import (
	"context"
	"time"

	"github.com/web3chat/txchat/dtalk/service/chatclean/config"
	"github.com/web3chat/txchat/dtalk/service/chatclean/dao"
)

// DateOption 手动删除时间范围选项（与 proto 保持一致）
type DateOption int32

const (
	DateOptionCustom     DateOption = 0
	DateOptionLast1Day   DateOption = 1
	DateOptionLast3Days  DateOption = 2
	DateOptionLast7Days  DateOption = 3
	DateOptionLast30Days DateOption = 4
	DateOptionLast90Days DateOption = 5
	DateOptionLast180Days DateOption = 6
	DateOptionLast365Days DateOption = 7
	DateOptionAllTime    DateOption = 8
)

// AutoDeleteOption 自动删除保留时长选项
type AutoDeleteOption int8

const (
	AutoDeleteNever   AutoDeleteOption = 0
	AutoDelete1Day    AutoDeleteOption = 1
	AutoDelete3Days   AutoDeleteOption = 2
	AutoDelete7Days   AutoDeleteOption = 3
	AutoDelete30Days  AutoDeleteOption = 4
	AutoDelete180Days AutoDeleteOption = 5
)

// optionToDuration 将 AutoDeleteOption 转换为对应的 Duration
var optionToDuration = map[AutoDeleteOption]time.Duration{
	AutoDelete1Day:    24 * time.Hour,
	AutoDelete3Days:   3 * 24 * time.Hour,
	AutoDelete7Days:   7 * 24 * time.Hour,
	AutoDelete30Days:  30 * 24 * time.Hour,
	AutoDelete180Days: 180 * 24 * time.Hour,
}

// dateOptionToRange 将 DateOption 转换为 [startTime, endTime]（Unix ms）
// endTime 固定为当前时间
func dateOptionToRange(opt DateOption, customStart, customEnd int64) (int64, int64) {
	now := time.Now().UnixMilli()
	switch opt {
	case DateOptionCustom:
		return customStart, customEnd
	case DateOptionLast1Day:
		return now - int64(24*time.Hour/time.Millisecond), now
	case DateOptionLast3Days:
		return now - int64(3*24*time.Hour/time.Millisecond), now
	case DateOptionLast7Days:
		return now - int64(7*24*time.Hour/time.Millisecond), now
	case DateOptionLast30Days:
		return now - int64(30*24*time.Hour/time.Millisecond), now
	case DateOptionLast90Days:
		return now - int64(90*24*time.Hour/time.Millisecond), now
	case DateOptionLast180Days:
		return now - int64(180*24*time.Hour/time.Millisecond), now
	case DateOptionLast365Days:
		return now - int64(365*24*time.Hour/time.Millisecond), now
	case DateOptionAllTime:
		return 0, now
	default:
		return 0, now
	}
}

// normalizeUidPair 保证 uid_a < uid_b，避免重复行
func normalizeUidPair(uid1, uid2 string) (string, string) {
	if uid1 <= uid2 {
		return uid1, uid2
	}
	return uid2, uid1
}

// Service 聊天清理业务逻辑层
type Service struct {
	dao *dao.Dao
	cfg *config.Config
}

func New(cfg *config.Config) *Service {
	return &Service{
		dao: dao.New(cfg),
		cfg: cfg,
	}
}

func (s *Service) Dao() *dao.Dao {
	return s.dao
}

// CleanPrivateChat 手动删除私聊记录
func (s *Service) CleanPrivateChat(ctx context.Context, operatorID, targetID string, opt DateOption, customStart, customEnd int64) (int64, error) {
	startTime, endTime := dateOptionToRange(opt, customStart, customEnd)
	uidA, uidB := normalizeUidPair(operatorID, targetID)
	deleted, err := s.dao.CleanPrivateChat(ctx, uidA, uidB, startTime, endTime)
	if err != nil {
		return 0, err
	}
	now := time.Now().UnixMilli()
	// 写审计日志，忽略失败（审计日志不影响主流程）
	_ = s.dao.WriteAuditLog(ctx, operatorID, targetID, 0, int8(opt), startTime, endTime, deleted, now)
	return deleted, nil
}

// SetAutoDeleteConfig 保存自动删除配置
func (s *Service) SetAutoDeleteConfig(ctx context.Context, operatorID, targetID string, opt AutoDeleteOption) (*dao.AutoDeleteConfig, error) {
	uidA, uidB := normalizeUidPair(operatorID, targetID)
	now := time.Now().UnixMilli()
	if err := s.dao.UpsertAutoDeleteConfig(ctx, uidA, uidB, int8(opt), operatorID, now); err != nil {
		return nil, err
	}
	return &dao.AutoDeleteConfig{
		UidA:      uidA,
		UidB:      uidB,
		Option:    int8(opt),
		UpdatedBy: operatorID,
		UpdatedAt: now,
	}, nil
}

// GetAutoDeleteConfig 查询自动删除配置
func (s *Service) GetAutoDeleteConfig(ctx context.Context, operatorID, targetID string) (*dao.AutoDeleteConfig, error) {
	uidA, uidB := normalizeUidPair(operatorID, targetID)
	cfg, err := s.dao.GetAutoDeleteConfig(ctx, uidA, uidB)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		// 没有配置视为"从不删除"
		return &dao.AutoDeleteConfig{
			UidA:   uidA,
			UidB:   uidB,
			Option: int8(AutoDeleteNever),
		}, nil
	}
	return cfg, nil
}

// RunAutoDeleteOnce 对一对用户执行一次自动删除（由调度器调用）
func (s *Service) RunAutoDeleteOnce(ctx context.Context, cfg *dao.AutoDeleteConfig) (int64, error) {
	opt := AutoDeleteOption(cfg.Option)
	dur, ok := optionToDuration[opt]
	if !ok {
		return 0, nil
	}
	endTime := time.Now().UnixMilli()
	startTime := int64(0)
	endTime = endTime - int64(dur/time.Millisecond)
	// 删除 [0, now-duration] 内的消息（即超出保留期的消息）
	deleted, err := s.dao.CleanPrivateChat(ctx, cfg.UidA, cfg.UidB, startTime, endTime)
	if err != nil {
		return 0, err
	}
	now := time.Now().UnixMilli()
	_ = s.dao.WriteAuditLog(ctx, "scheduler", cfg.UidB, 1, cfg.Option, startTime, endTime, deleted, now)
	return deleted, nil
}

func (s *Service) Shutdown(_ context.Context) {
	s.dao.Close()
}
