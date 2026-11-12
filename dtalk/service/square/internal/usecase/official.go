package usecase

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	"github.com/web3chat/txchat/dtalk/service/square/internal/repository"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

func (s *Service) OfficialMe(ctx context.Context, addr string) map[string]any {
	oa, err := s.store.GetOfficial(ctx, addr)
	if err != nil {
		s.log.Error("official me failed", "addr", addr, "err", err)
	}
	apply, _ := s.store.LatestApply(ctx, addr)
	state := "none"
	switch {
	case oa != nil && oa.Status == entity.OAStatusBanned:
		state = "banned"
	case oa != nil && oa.Status == entity.OAStatusNormal:
		state = "approved"
	case apply != nil && (apply.Status == entity.ApplyPending || apply.Status == entity.ApplyRejected):
		state = apply.Status
	}
	out := map[string]any{"state": state, "address": addr, "userBanned": false, "square": s.SquareStatus(ctx, addr)}
	ban, banErr := s.store.GetUserBan(ctx, addr)
	if banErr != nil {
		s.log.Error("official me user ban lookup failed", "addr", addr, "err", banErr)
	} else if ban != nil {
		out["userBanned"] = true
	}
	if oa != nil {
		out["official"] = map[string]any{
			"name": oa.Name, "avatar": oa.Avatar, "intro": oa.Intro, "status": oa.Status, "gold": oa.Gold,
		}
	}
	if apply != nil {
		out["apply"] = map[string]any{
			"id": apply.ID, "status": apply.Status, "name": apply.Name, "remark": apply.ReviewRemark,
		}
	}
	return out
}

func (s *Service) ApplyOfficial(ctx context.Context, addr, name, avatar, intro string) (*entity.OfficialApply, error) {
	if err := s.AssertSquareReadable(ctx); err != nil {
		return nil, err
	}
	if err := s.AssertNotBanned(ctx, addr); err != nil {
		return nil, err
	}
	oa, err := s.store.GetOfficial(ctx, addr)
	if err != nil {
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if oa != nil && oa.Status == entity.OAStatusBanned {
		return nil, xerror.New(xerror.OfficialBanned, "公众号已封禁")
	}
	if oa != nil && oa.Status == entity.OAStatusNormal {
		return nil, xerror.New(xerror.ParamInvalid, "已是认证公众号")
	}
	pending, err := s.store.GetPendingApply(ctx, addr)
	if err != nil {
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if pending != nil {
		return nil, xerror.New(xerror.AlreadyPending, "已有待审申请")
	}
	name, err = normalizeOfficialName(name)
	if err != nil {
		return nil, err
	}
	if utf8.RuneCountInString(intro) > 200 {
		return nil, xerror.New(xerror.ParamInvalid, "简介过长")
	}
	if err := s.rejectTakenOfficialName(ctx, name, 0, addr); err != nil {
		return nil, err
	}
	if err := s.CheckRate(ctx, addr, entity.ActionOfficialApply); err != nil {
		return nil, err
	}
	a := &entity.OfficialApply{
		Address: addr, Name: name, Avatar: strings.TrimSpace(avatar), Intro: strings.TrimSpace(intro), Status: entity.ApplyPending,
	}
	if _, err := s.store.InsertApply(ctx, a); err != nil {
		if errors.Is(err, repository.ErrOfficialNameTaken) {
			s.log.Warn("official apply name taken", "addr", addr, "name", name)
			return nil, xerror.New(xerror.OfficialNameTaken, "公众号名称已被使用")
		}
		s.log.Error("insert apply failed", "addr", addr, "name", name, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	return a, nil
}

func (s *Service) rejectTakenOfficialName(ctx context.Context, name string, exceptApplyID int64, addr string) error {
	return s.rejectOfficialName(ctx, name, exceptApplyID, addr, true)
}

func (s *Service) rejectGrantedOfficialName(ctx context.Context, name string, exceptApplyID int64, addr string) error {
	return s.rejectOfficialName(ctx, name, exceptApplyID, addr, false)
}

func (s *Service) rejectOfficialName(ctx context.Context, name string, exceptApplyID int64, addr string, includePending bool) error {
	var taken bool
	var err error
	if includePending {
		taken, err = s.store.OfficialNameTaken(ctx, name, exceptApplyID)
	} else {
		taken, err = s.store.OfficialNameGranted(ctx, name, exceptApplyID)
	}
	if err != nil {
		s.log.Error("check official name failed", "addr", addr, "name", name, "exceptApply", exceptApplyID, "includePending", includePending, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if !taken {
		return nil
	}
	s.log.Warn("official name taken", "addr", addr, "name", name, "exceptApply", exceptApplyID, "includePending", includePending)
	return xerror.New(xerror.OfficialNameTaken, "公众号名称已被使用")
}

func (s *Service) ReviewApply(ctx context.Context, id int64, approve bool, reviewer, remark string) error {
	a, err := s.store.GetApply(ctx, id)
	if err != nil {
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if a == nil {
		return xerror.New(xerror.NotFound, "申请不存在")
	}
	if a.Status != entity.ApplyPending {
		return xerror.New(xerror.ParamInvalid, "申请已处理")
	}
	st := entity.ApplyRejected
	if approve {
		if err := s.rejectGrantedOfficialName(ctx, a.Name, a.ID, a.Address); err != nil {
			return err
		}
		st = entity.ApplyApproved
	}
	if err := s.store.ReviewApply(ctx, id, st, reviewer, remark); err != nil {
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if approve {
		oa := &entity.OfficialAccount{
			Address: a.Address, Name: a.Name, Avatar: a.Avatar, Intro: a.Intro,
			Status: entity.OAStatusNormal, ApplyID: a.ID,
		}
		if err := s.store.UpsertOfficial(ctx, oa); err != nil {
			s.log.Error("upsert official failed", "addr", a.Address, "err", err)
			return xerror.Wrap(xerror.Internal, "内部错误", err)
		}
	}
	return nil
}

func (s *Service) ListApply(ctx context.Context, statuses []string, page entity.PageQuery) ([]entity.OfficialApply, int64, error) {
	return s.store.ListApply(ctx, statuses, page)
}

func (s *Service) ListOfficial(ctx context.Context, f repository.OfficialListFilter, page entity.PageQuery) ([]entity.OfficialAccount, int64, error) {
	list, total, err := s.store.ListOfficial(ctx, f, page)
	if err != nil {
		s.log.Error("list official failed", "statuses", f.Statuses, "golds", f.Golds, "order", f.Order, "err", err)
		return nil, 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	return list, total, nil
}

func (s *Service) ListAppOfficials(ctx context.Context, viewer string, golds []bool, page entity.PageQuery) ([]entity.OfficialAccount, int64, error) {
	if err := s.AssertSquareReadable(ctx); err != nil {
		return nil, 0, err
	}
	if len(golds) == 0 {
		golds = []bool{true}
	}
	list, total, err := s.ListOfficial(ctx, repository.OfficialListFilter{
		Statuses: []int16{entity.OAStatusNormal},
		Golds:    golds,
		Order:    "followers",
	}, page)
	if err != nil {
		return nil, 0, err
	}
	s.markOfficialFollowed(ctx, viewer, list)
	return list, total, nil
}

func (s *Service) markOfficialFollowed(ctx context.Context, viewer string, list []entity.OfficialAccount) {
	addrs := make([]string, 0, len(list))
	for i := range list {
		addrs = append(addrs, list[i].Address)
	}
	set, err := s.store.FollowedSet(ctx, viewer, addrs)
	if err != nil {
		s.log.Error("official followed set failed", "viewer", viewer, "n", len(addrs), "err", err)
		return
	}
	for i := range list {
		_, list[i].Followed = set[list[i].Address]
	}
}

func (s *Service) SetOfficialGold(ctx context.Context, addr, operator string, gold bool) error {
	oa, err := s.store.GetOfficial(ctx, addr)
	if err != nil {
		s.log.Error("get official for gold failed", "addr", addr, "operator", operator, "gold", gold, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if oa == nil {
		s.log.Error("set official gold missing", "addr", addr, "operator", operator, "gold", gold)
		return xerror.New(xerror.NotFound, "公众号不存在")
	}
	if err := s.store.SetOfficialGold(ctx, addr, gold); err != nil {
		s.log.Error("set official gold failed", "addr", addr, "operator", operator, "gold", gold, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.log.Info("official gold updated", "addr", addr, "operator", operator, "gold", gold, "name", oa.Name)
	return nil
}

func (s *Service) BanOfficial(ctx context.Context, addr, reason string, hidePosts bool) error {
	oa, err := s.store.GetOfficial(ctx, addr)
	if err != nil {
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if oa == nil {
		return xerror.New(xerror.NotFound, "公众号不存在")
	}
	if addr == entity.PlatformOfficialAddr {
		s.log.Warn("refuse ban platform official", "addr", addr, "reason", reason)
		return xerror.New(xerror.ParamInvalid, "平台官方账号不可封禁")
	}
	if err := s.store.BanOfficial(ctx, addr, reason); err != nil {
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if hidePosts {
		if err := s.store.HidePostsByAuthor(ctx, addr); err != nil {
			s.log.Error("hide posts on ban failed", "addr", addr, "err", err)
		}
	}
	return nil
}

func (s *Service) RevokeOfficial(ctx context.Context, addr, operator string) error {
	oa, err := s.store.GetOfficial(ctx, addr)
	if err != nil {
		s.log.Error("get official for revoke failed", "addr", addr, "operator", operator, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if oa == nil {
		s.log.Warn("revoke official missing", "addr", addr, "operator", operator)
		return xerror.New(xerror.NotFound, "公众号不存在")
	}
	if addr == entity.PlatformOfficialAddr {
		s.log.Warn("refuse revoke platform official", "addr", addr, "operator", operator)
		return xerror.New(xerror.ParamInvalid, "平台官方账号不可取消")
	}
	posts, err := s.store.RevokeOfficial(ctx, addr)
	if err != nil {
		s.log.Error("revoke official failed", "addr", addr, "operator", operator, "name", oa.Name, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.log.Info("official revoked", "addr", addr, "operator", operator, "name", oa.Name, "posts", posts)
	return nil
}

func (s *Service) UnbanOfficial(ctx context.Context, addr string) error {
	oa, err := s.store.GetOfficial(ctx, addr)
	if err != nil {
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if oa == nil {
		return xerror.New(xerror.NotFound, "公众号不存在")
	}
	return s.store.UnbanOfficial(ctx, addr)
}

func (s *Service) UpdateOfficialProfile(ctx context.Context, addr, name, avatar string) error {
	oa, err := s.store.GetOfficial(ctx, addr)
	if err != nil {
		s.log.Error("get official for profile update failed", "addr", addr, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if oa == nil {
		return xerror.New(xerror.NotFound, "公众号不存在")
	}
	name, err = normalizeOfficialName(name)
	if err != nil {
		return err
	}
	avatar, err = normalizeOfficialAvatar(avatar)
	if err != nil {
		return err
	}
	if err := s.store.UpdateOfficialProfile(ctx, addr, name, avatar); err != nil {
		s.log.Error("update official profile failed", "addr", addr, "name", name, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if err := s.store.UpdateOfficialPostNames(ctx, addr, name); err != nil {
		s.log.Error("update official post names failed", "addr", addr, "name", name, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	return nil
}

func (s *Service) UpdateMyOfficial(ctx context.Context, addr, name, avatar string) error {
	if err := s.AssertNotBanned(ctx, addr); err != nil {
		return err
	}
	return s.UpdateOfficialProfile(ctx, addr, name, avatar)
}

func normalizeOfficialName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 32 {
		return "", xerror.New(xerror.ParamInvalid, "公众号名称不合法")
	}
	return name, nil
}

func normalizeOfficialAvatar(avatar string) (string, error) {
	avatar = strings.TrimSpace(avatar)
	if utf8.RuneCountInString(avatar) > 2048 {
		return "", xerror.New(xerror.ParamInvalid, "头像地址过长")
	}
	return avatar, nil
}

func (s *Service) AdminGetPost(ctx context.Context, id int64) (*entity.Post, error) {
	p, err := s.store.GetPost(ctx, id)
	if err != nil {
		s.log.Error("admin get post failed", "id", id, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if p == nil {
		return nil, xerror.New(xerror.NotFound, "帖子不存在")
	}
	s.fillAuthorAvatar(ctx, p)
	return p, nil
}

func (s *Service) AdminListOfficialPostReplies(ctx context.Context, page entity.PageQuery) ([]entity.OfficialPostReply, int64, error) {
	list, total, err := s.store.ListOfficialPostReplies(ctx, page)
	if err != nil {
		s.log.Error("admin list official post replies failed", "page", page.Page, "pageSize", page.PageSize, "err", err)
		return nil, 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	for i := range list {
		list[i].PostTitle = PostTitleSnapshot(list[i].Title, list[i].Intro, list[i].Content)
	}
	return list, total, nil
}

func (s *Service) AdminListPosts(ctx context.Context, f repository.PostListFilter, page entity.PageQuery) ([]entity.Post, int64, error) {
	if IsRankOrder(f.Order) && f.From == nil {
		f.From = RankWindowStart("week", time.Now())
	}
	list, total, err := s.store.ListPosts(ctx, f, page)
	if err != nil {
		from := ""
		if f.From != nil {
			from = f.From.Format(time.RFC3339)
		}
		s.log.Error("admin list posts failed", "order", f.Order, "from", from, "addrs", f.AuthorAddrs, "names", f.AuthorNames, "tags", f.Tags, "err", err)
		return nil, 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.fillAuthorAvatars(ctx, list)
	return list, total, nil
}

func (s *Service) AdminListComments(ctx context.Context, f repository.CommentListFilter, page entity.PageQuery) ([]entity.Comment, int64, error) {
	list, total, err := s.store.ListComments(ctx, f, page)
	if err != nil {
		s.log.Error("admin list comments failed", "order", f.Order, "postId", f.PostID, "err", err)
		return nil, 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	return list, total, nil
}

func (s *Service) AdminSetPostStatus(ctx context.Context, id int64, status int16) error {
	p, err := s.store.GetPost(ctx, id)
	if err != nil {
		s.log.Error("admin get post for status failed", "id", id, "status", status, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if p == nil {
		return xerror.New(xerror.NotFound, "帖子不存在")
	}
	if err := s.store.UpdatePostStatus(ctx, id, status); err != nil {
		s.log.Error("admin set post status failed", "id", id, "status", status, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	return nil
}

const maxBatchPostIDs = 100

func (s *Service) AdminBatchDeletePosts(ctx context.Context, ids []int64) ([]int64, error) {
	ids, err := normalizeBatchIDs(ids, maxBatchPostIDs)
	if err != nil {
		return nil, err
	}
	updated, err := s.store.UpdatePostStatusMany(ctx, ids, entity.StatusDeleted)
	if err != nil {
		s.log.Error("admin batch delete posts failed", "ids", ids, "n", len(ids), "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.log.Info("admin batch delete posts", "requested", len(ids), "updated", len(updated), "ids", updated)
	return updated, nil
}

func normalizeBatchIDs(ids []int64, max int) ([]int64, error) {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, xerror.New(xerror.ParamInvalid, "请选择要删除的帖子")
	}
	if max > 0 && len(out) > max {
		return nil, xerror.New(xerror.ParamInvalid, "一次最多删除 100 条")
	}
	return out, nil
}

func (s *Service) AdminSetCommentStatus(ctx context.Context, id int64, status int16) error {
	c, err := s.store.GetComment(ctx, id)
	if err != nil {
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if c == nil {
		return xerror.New(xerror.NotFound, "评论不存在")
	}
	wasVisible := c.Status == entity.StatusVisible
	if err := s.commitCommentStatus(ctx, c, wasVisible, status, nil); err != nil {
		s.log.Error("admin set comment status failed", "id", id, "status", status, "err", err)
		return err
	}
	return nil
}

func (s *Service) OverrideCommentAI(ctx context.Context, id int64, username string, pass bool) error {
	c, err := s.store.GetComment(ctx, id)
	if err != nil {
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if c == nil {
		return xerror.New(xerror.NotFound, "评论不存在")
	}
	st := entity.StatusAIReject
	verdict := entity.VerdictReject
	if pass {
		st = entity.StatusVisible
		verdict = entity.VerdictPass
	}
	wasVisible := c.Status == entity.StatusVisible
	if err := s.commitCommentStatus(ctx, c, wasVisible, st, nil); err != nil {
		s.log.Error("override comment ai failed", "id", id, "pass", pass, "user", username, "err", err)
		return err
	}
	m, _ := s.store.LatestModeration(ctx, entity.TargetComment, id)
	if m != nil {
		if err := s.store.OverrideModeration(ctx, m.ID, username, verdict); err != nil {
			s.log.Error("override comment moderation failed", "id", id, "moderation", m.ID, "err", err)
		}
	}
	return nil
}

func (s *Service) OverridePostAI(ctx context.Context, id int64, username string, pass bool) error {
	p, err := s.store.GetPost(ctx, id)
	if err != nil {
		s.log.Error("admin get post for ai override failed", "id", id, "pass", pass, "user", username, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if p == nil {
		return xerror.New(xerror.NotFound, "帖子不存在")
	}
	st := entity.StatusAIReject
	verdict := entity.VerdictReject
	if pass {
		st = entity.StatusVisible
		verdict = entity.VerdictPass
	}
	if err := s.store.UpdatePostStatus(ctx, id, st); err != nil {
		s.log.Error("override post ai failed", "id", id, "pass", pass, "user", username, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	m, _ := s.store.LatestModeration(ctx, entity.TargetPost, id)
	if m != nil {
		if err := s.store.OverrideModeration(ctx, m.ID, username, verdict); err != nil {
			s.log.Error("override post moderation failed", "id", id, "moderation", m.ID, "err", err)
		}
	}
	return nil
}

func (s *Service) LatestModeration(ctx context.Context, target string, id int64) (*entity.Moderation, error) {
	return s.store.LatestModeration(ctx, target, id)
}

func (s *Service) LatestModerationMap(ctx context.Context, target string, ids []int64) (map[int64]*entity.Moderation, error) {
	m, err := s.store.LatestModerationMap(ctx, target, ids)
	if err != nil {
		s.log.Error("latest moderation map failed", "type", target, "n", len(ids), "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	return m, nil
}
