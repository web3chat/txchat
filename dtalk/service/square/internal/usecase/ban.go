package usecase

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

const maxBanReasonRunes = 500

func (s *Service) BanUser(ctx context.Context, addr, reason, operator string, hidePosts, hideComments bool) error {
	addr, err := normalizeBanAddr(addr)
	if err != nil {
		return err
	}
	if addr == entity.PlatformOfficialAddr {
		s.log.Warn("refuse ban platform official user", "addr", addr, "operator", operator)
		return xerror.New(xerror.ParamInvalid, "平台官方账号不可封禁")
	}
	reason, err = normalizeBanReason(reason)
	if err != nil {
		return err
	}
	if err := s.store.UpsertUserBan(ctx, &entity.UserBan{
		Address:  addr,
		Reason:   reason,
		Operator: strings.TrimSpace(operator),
	}); err != nil {
		s.log.Error("upsert user ban failed", "addr", addr, "operator", operator, "reason", reason, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.log.Info("user banned", "addr", addr, "operator", operator, "reason", reason, "hidePosts", hidePosts, "hideComments", hideComments)
	if hidePosts {
		if err := s.store.HidePostsByAuthor(ctx, addr); err != nil {
			s.log.Error("hide posts on user ban failed", "addr", addr, "operator", operator, "err", err)
		}
	}
	if hideComments {
		if err := s.store.HideCommentsByAuthor(ctx, addr); err != nil {
			s.log.Error("hide comments on user ban failed", "addr", addr, "operator", operator, "err", err)
		}
	}
	return nil
}

func (s *Service) UnbanUser(ctx context.Context, addr, operator string) error {
	addr, err := normalizeBanAddr(addr)
	if err != nil {
		return err
	}
	ok, err := s.store.DeleteUserBan(ctx, addr)
	if err != nil {
		s.log.Error("delete user ban failed", "addr", addr, "operator", operator, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if !ok {
		s.log.Warn("unban user not found", "addr", addr, "operator", operator)
		return xerror.New(xerror.NotFound, "该用户未被封禁")
	}
	s.log.Info("user unbanned", "addr", addr, "operator", operator)
	return nil
}

func (s *Service) ListUserBans(ctx context.Context, addrs []string, order string, page entity.PageQuery) ([]entity.UserBan, int64, error) {
	list, total, err := s.store.ListUserBans(ctx, addrs, order, page)
	if err != nil {
		s.log.Error("list user bans failed", "addrs", addrs, "order", order, "err", err)
		return nil, 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.log.Info("list user bans", "addrs", addrs, "order", order, "total", total, "page", page.Page, "size", page.PageSize)
	return list, total, nil
}

func (s *Service) UserBanSet(ctx context.Context, addrs []string) map[string]bool {
	raw, err := s.store.UserBanMap(ctx, addrs)
	if err != nil {
		s.log.Error("user ban map failed", "addrs", addrs, "err", err)
		return map[string]bool{}
	}
	out := make(map[string]bool, len(raw))
	for addr := range raw {
		out[addr] = true
	}
	return out
}

func BanAuditExtra(reason string, hidePosts, hideComments bool) string {
	reason = strings.TrimSpace(reason)
	flags := fmt.Sprintf("hidePosts=%t hideComments=%t", hidePosts, hideComments)
	if reason == "" {
		return flags
	}
	return reason + " | " + flags
}

func normalizeBanAddr(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" || utf8.RuneCountInString(addr) > 64 {
		return "", xerror.New(xerror.ParamInvalid, "钱包地址不合法")
	}
	return addr, nil
}

func normalizeBanReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) > maxBanReasonRunes {
		return "", xerror.New(xerror.ParamInvalid, "封禁原因过长")
	}
	return reason, nil
}
