package usecase

import (
	"context"
	"strings"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

func normalizeFollowTarget(follower, target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" || len(target) > 64 {
		return "", xerror.New(xerror.ParamInvalid, "参数错误")
	}
	if follower == "" {
		return "", xerror.New(xerror.ParamInvalid, "参数错误")
	}
	if follower == target {
		return "", xerror.New(xerror.FollowSelf, "不能关注自己")
	}
	return target, nil
}

func (s *Service) Follow(ctx context.Context, follower, target string) error {
	if err := s.AssertNotBanned(ctx, follower); err != nil {
		return err
	}
	target, err := normalizeFollowTarget(follower, target)
	if err != nil {
		return err
	}
	if err := s.CheckRate(ctx, follower, entity.ActionFollow); err != nil {
		return err
	}
	added, err := s.store.AddFollow(ctx, follower, target)
	if err != nil {
		s.log.Error("follow failed", "follower", follower, "target", target, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.log.Info("follow", "follower", follower, "target", target, "added", added)
	return nil
}

func (s *Service) Unfollow(ctx context.Context, follower, target string) error {
	target, err := normalizeFollowTarget(follower, target)
	if err != nil {
		return err
	}
	removed, err := s.store.RemoveFollow(ctx, follower, target)
	if err != nil {
		s.log.Error("unfollow failed", "follower", follower, "target", target, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.log.Info("unfollow", "follower", follower, "target", target, "removed", removed)
	return nil
}

func (s *Service) ListMyFollows(ctx context.Context, addr string, page entity.PageQuery) ([]entity.FollowProfile, int64, error) {
	list, total, err := s.store.ListFollows(ctx, addr, page)
	if err != nil {
		s.log.Error("list follows failed", "addr", addr, "page", page.Page, "err", err)
		return nil, 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	return list, total, nil
}

func (s *Service) ListMyFollowers(ctx context.Context, addr string, page entity.PageQuery) ([]entity.FollowProfile, int64, error) {
	list, total, err := s.store.ListFollowers(ctx, addr, page)
	if err != nil {
		s.log.Error("list followers failed", "addr", addr, "page", page.Page, "err", err)
		return nil, 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	return list, total, nil
}
