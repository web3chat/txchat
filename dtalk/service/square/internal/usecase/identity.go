package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

const officialPostDayLimit int64 = 10

type rateWindow struct {
	dur   time.Duration
	limit int64
}

// postRateWindows 普通用户每天 20 条。已入驻公众号每天 10 条。金标公众号不限发帖次数。
func postRateWindows(official, gold bool) []rateWindow {
	if official && gold {
		return nil
	}
	day := int64(20)
	if official {
		day = officialPostDayLimit
	}
	return []rateWindow{{10 * time.Minute, 5}, {24 * time.Hour, day}}
}

func (s *Service) CheckRate(ctx context.Context, addr, action string) error {
	now := time.Now()
	var windows []rateWindow
	switch action {
	case entity.ActionPost:
		oa, err := s.store.GetOfficial(ctx, addr)
		if err != nil {
			s.log.Error("get official for post rate failed", "addr", addr, "err", err)
			return xerror.Wrap(xerror.Internal, "内部错误", err)
		}
		official := oa != nil && oa.Status == entity.OAStatusNormal
		gold := official && oa.Gold
		windows = postRateWindows(official, gold)
		if gold {
			s.log.Info("gold official post rate skipped", "addr", addr, "name", oa.Name)
		}
	case entity.ActionComment:
		windows = []rateWindow{{10 * time.Minute, 20}, {24 * time.Hour, 100}}
	case entity.ActionLike:
		windows = []rateWindow{{time.Minute, 60}}
	case entity.ActionOfficialApply:
		windows = []rateWindow{{24 * time.Hour, 3}}
	case entity.ActionFollow:
		windows = []rateWindow{{time.Minute, 30}}
	case entity.ActionReport:
		windows = []rateWindow{{10 * time.Minute, 10}, {24 * time.Hour, 20}}
	case entity.ActionEdit:
		windows = []rateWindow{{10 * time.Minute, 10}, {24 * time.Hour, 40}}
	default:
		return nil
	}
	for _, w := range windows {
		n, err := s.store.CountRate(ctx, addr, action, now.Add(-w.dur))
		if err != nil {
			s.log.Error("count rate failed", "addr", addr, "action", action, "err", err)
			return xerror.Wrap(xerror.Internal, "内部错误", err)
		}
		if n >= w.limit {
			msg := "操作过于频繁"
			if action == entity.ActionPost && w.dur == 24*time.Hour && w.limit == officialPostDayLimit {
				msg = fmt.Sprintf("公众号每日最多发布 %d 条", officialPostDayLimit)
			}
			s.log.Warn("rate limited", "addr", addr, "action", action, "count", n, "limit", w.limit, "window", w.dur.String())
			return xerror.New(xerror.RateLimited, msg)
		}
	}
	if err := s.store.HitRate(ctx, addr, action); err != nil {
		s.log.Error("hit rate failed", "addr", addr, "action", action, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	return nil
}

func (s *Service) DisplayName(ctx context.Context, addr, nickname string) (name string, kind int16, err error) {
	oa, err := s.store.GetOfficial(ctx, addr)
	if err != nil {
		s.log.Error("get official for display", "addr", addr, "err", err)
		return "", 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if oa != nil && oa.Status == entity.OAStatusBanned {
		return "", 0, xerror.New(xerror.OfficialBanned, "公众号已封禁")
	}
	if oa != nil && oa.Status == entity.OAStatusNormal {
		return oa.Name, entity.KindOfficial, nil
	}
	if nickname == "" {
		nickname = addr
	}
	return nickname, entity.KindPersonal, nil
}

func (s *Service) AssertNotBanned(ctx context.Context, addr string) error {
	ban, err := s.store.GetUserBan(ctx, addr)
	if err != nil {
		s.log.Error("get user ban failed", "addr", addr, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if ban != nil {
		s.log.Warn("user banned", "addr", addr, "reason", ban.Reason, "operator", ban.Operator)
		return xerror.New(xerror.UserBanned, "账号已被封禁")
	}
	oa, err := s.store.GetOfficial(ctx, addr)
	if err != nil {
		s.log.Error("get official for ban check failed", "addr", addr, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if oa != nil && oa.Status == entity.OAStatusBanned {
		return xerror.New(xerror.OfficialBanned, "公众号已封禁")
	}
	return nil
}
