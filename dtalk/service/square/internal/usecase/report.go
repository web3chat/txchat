package usecase

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	"github.com/web3chat/txchat/dtalk/service/square/internal/repository"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

const maxReportDetail = 500

func validReportReason(reason string) bool {
	switch reason {
	case entity.ReportSpam, entity.ReportPorn, entity.ReportScam, entity.ReportAbuse, entity.ReportOther:
		return true
	default:
		return false
	}
}

func (s *Service) ReportPost(ctx context.Context, addr string, postID int64, reason, detail string) (*entity.Report, error) {
	if err := s.AssertNotBanned(ctx, addr); err != nil {
		return nil, err
	}
	if postID <= 0 {
		return nil, xerror.New(xerror.ParamInvalid, "参数错误")
	}
	reason = strings.TrimSpace(reason)
	if !validReportReason(reason) {
		return nil, xerror.New(xerror.ParamInvalid, "举报原因不合法")
	}
	detail = strings.TrimSpace(detail)
	if utf8.RuneCountInString(detail) > maxReportDetail {
		return nil, xerror.New(xerror.ParamInvalid, "举报说明过长")
	}
	p, err := s.requireVisiblePost(ctx, postID)
	if err != nil {
		return nil, err
	}
	if err := s.CheckRate(ctx, addr, entity.ActionReport); err != nil {
		return nil, err
	}
	r := &entity.Report{
		ReporterAddr: addr,
		TargetType:   entity.TargetPost,
		TargetID:     p.ID,
		Reason:       reason,
		Detail:       detail,
		Status:       entity.ReportPending,
	}
	if _, err := s.store.InsertReport(ctx, r); err != nil {
		if repository.IsAlreadyReported(err) {
			s.log.Warn("duplicate report", "addr", addr, "post", postID, "reason", reason)
			return nil, xerror.New(xerror.AlreadyReported, "已举报过该内容")
		}
		s.log.Error("insert report failed", "addr", addr, "post", postID, "reason", reason, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.log.Info("post reported", "id", r.ID, "addr", addr, "post", postID, "reason", reason)
	return r, nil
}

func (s *Service) AdminListReports(ctx context.Context, f repository.ReportListFilter, page entity.PageQuery) ([]entity.Report, int64, error) {
	list, total, err := s.store.ListReports(ctx, f, page)
	if err != nil {
		s.log.Error("admin list reports failed", "statuses", f.Statuses, "reasons", f.Reasons, "reporters", f.ReporterAddrs, "err", err)
		return nil, 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	return list, total, nil
}

func (s *Service) AdminHandleReport(ctx context.Context, id int64, action, handler string) error {
	r, err := s.store.GetReport(ctx, id)
	if err != nil {
		s.log.Error("get report failed", "id", id, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if r == nil {
		return xerror.New(xerror.NotFound, "举报不存在")
	}
	if r.Status != entity.ReportPending {
		return xerror.New(xerror.ParamInvalid, "举报已处理")
	}
	action = strings.ToLower(strings.TrimSpace(action))
	st := entity.ReportHandled
	switch action {
	case "hide":
		if err := s.AdminSetPostStatus(ctx, r.TargetID, entity.StatusHidden); err != nil {
			return err
		}
	case "delete":
		if err := s.AdminSetPostStatus(ctx, r.TargetID, entity.StatusDeleted); err != nil {
			return err
		}
	case "dismiss":
		st = entity.ReportDismissed
	default:
		return xerror.New(xerror.ParamInvalid, "处理动作不合法")
	}
	if err := s.store.HandleReport(ctx, id, st, handler); err != nil {
		s.log.Error("handle report failed", "id", id, "action", action, "handler", handler, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.log.Info("report handled", "id", id, "action", action, "handler", handler, "post", r.TargetID, "reporter", r.ReporterAddr)
	return nil
}

func (s *Service) AdminListAccounts(ctx context.Context, addrs, names []string, page entity.PageQuery) ([]entity.AccountStat, int64, error) {
	list, total, err := s.store.ListAccounts(ctx, addrs, names, page)
	if err != nil {
		s.log.Error("admin list accounts failed", "addrs", addrs, "names", names, "err", err)
		return nil, 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	return list, total, nil
}
