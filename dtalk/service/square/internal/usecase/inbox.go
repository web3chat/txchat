package usecase

import (
	"context"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	"github.com/web3chat/txchat/dtalk/service/square/internal/repository"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

func (s *Service) ListReplyInbox(ctx context.Context, addr string, read *bool, page entity.PageQuery) ([]entity.ReplyNotice, int64, int64, error) {
	list, total, err := s.store.ListReplyNotices(ctx, addr, repository.ReplyNoticeFilter{Read: read}, page)
	if err != nil {
		s.log.Error("list reply inbox failed", "addr", addr, "err", err)
		return nil, 0, 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	unread, err := s.store.CountDisplayableUnread(ctx, addr)
	if err != nil {
		s.log.Error("count reply inbox unread failed", "addr", addr, "err", err)
		return nil, 0, 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	for i := range list {
		if list[i].CommentStatus != entity.StatusVisible {
			list[i].Content = ""
		}
	}
	return list, total, unread, nil
}

func (s *Service) MarkReplyInboxRead(ctx context.Context, addr string, all bool, ids []int64) (int64, error) {
	if !all && len(ids) == 0 {
		return 0, xerror.New(xerror.ParamInvalid, "参数错误")
	}
	if err := s.store.MarkReplyNoticesRead(ctx, addr, all, ids); err != nil {
		s.log.Error("mark reply inbox read failed", "addr", addr, "all", all, "ids", ids, "err", err)
		return 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	unread, err := s.store.CountDisplayableUnread(ctx, addr)
	if err != nil {
		s.log.Error("count reply inbox unread after read failed", "addr", addr, "err", err)
		return 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	return unread, nil
}
