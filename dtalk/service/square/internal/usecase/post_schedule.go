package usecase

import (
	"context"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

const (
	minScheduleAhead   = time.Minute
	maxScheduleAhead   = 14 * 24 * time.Hour
	minExpireAfterSec  = int64(time.Hour / time.Second)
	maxExpireAfterSec  = int64(30 * 24 * time.Hour / time.Second)
	officialSweepLimit = 100
)

// OfficialTiming 是官方帖写入时的发布与删除时间。
type OfficialTiming struct {
	Status         int16
	PublishAt      *time.Time
	ExpireAt       *time.Time
	ExpireAfterSec int64
	Scheduled      bool
}

// ResolveOfficialTiming 校验定时发布和自动删除。live 表示帖子已经在广场上。
// 定时帖保持草稿，自动删除从实际发布时起算；已发布帖的删除从现在起算。
func ResolveOfficialTiming(now time.Time, live, draft bool, publishAt *time.Time, expireAfterSec int64) (OfficialTiming, error) {
	if expireAfterSec < 0 || expireAfterSec > maxExpireAfterSec || (expireAfterSec > 0 && expireAfterSec < minExpireAfterSec) {
		return OfficialTiming{}, xerror.New(xerror.ParamInvalid, "自动删除时间无效")
	}
	if live {
		if publishAt != nil {
			return OfficialTiming{}, xerror.New(xerror.ParamInvalid, "已发布帖不能再定时")
		}
		out := OfficialTiming{Status: entity.StatusVisible, ExpireAfterSec: expireAfterSec}
		if expireAfterSec > 0 {
			t := now.Add(time.Duration(expireAfterSec) * time.Second)
			out.ExpireAt = &t
		}
		return out, nil
	}
	if publishAt != nil {
		if err := validatePublishAt(now, *publishAt); err != nil {
			return OfficialTiming{}, err
		}
		t := publishAt.UTC()
		return OfficialTiming{
			Status:         entity.StatusDraft,
			PublishAt:      &t,
			ExpireAfterSec: expireAfterSec,
			Scheduled:      true,
		}, nil
	}
	if draft {
		return OfficialTiming{Status: entity.StatusDraft, ExpireAfterSec: expireAfterSec}, nil
	}
	out := OfficialTiming{Status: entity.StatusVisible, ExpireAfterSec: expireAfterSec}
	if expireAfterSec > 0 {
		t := now.Add(time.Duration(expireAfterSec) * time.Second)
		out.ExpireAt = &t
	}
	return out, nil
}

func validatePublishAt(now, at time.Time) error {
	if !at.After(now.Add(minScheduleAhead)) {
		return xerror.New(xerror.ParamInvalid, "定时发布至少要在 1 分钟之后")
	}
	if at.After(now.Add(maxScheduleAhead)) {
		return xerror.New(xerror.ParamInvalid, "定时发布不能超过两周")
	}
	return nil
}

func (s *Service) SweepOfficialSchedule(ctx context.Context) (published int, expired int, err error) {
	ids, err := s.store.PublishDueOfficialPosts(ctx, officialSweepLimit)
	if err != nil {
		s.log.Error("publish due official posts failed", "err", err)
		return 0, 0, err
	}
	if len(ids) > 0 {
		s.log.Info("official posts published on schedule", "n", len(ids), "ids", ids)
	}
	list, err := s.store.ExpireDueOfficialPosts(ctx, officialSweepLimit)
	if err != nil {
		s.log.Error("expire due official posts failed", "err", err)
		return len(ids), 0, err
	}
	if len(list) > 0 {
		expiredIDs := make([]int64, 0, len(list))
		for i := range list {
			expiredIDs = append(expiredIDs, list[i].ID)
			s.deleteExpiredMedia(ctx, &list[i])
		}
		s.log.Info("official posts expired", "n", len(list), "ids", expiredIDs)
	}
	return len(ids), len(list), nil
}
