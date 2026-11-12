package usecase

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	"github.com/web3chat/txchat/dtalk/service/square/internal/repository"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

func (s *Service) CreateComment(ctx context.Context, postID int64, addr, nickname, content string, replyToID int64) (*entity.Comment, error) {
	if err := s.AssertSquareReadable(ctx); err != nil {
		return nil, err
	}
	if err := s.AssertNotBanned(ctx, addr); err != nil {
		return nil, err
	}
	p, err := s.requireVisiblePost(ctx, postID)
	if err != nil {
		return nil, err
	}
	if p.CommentsEnabled == 0 {
		return nil, xerror.New(xerror.CommentsOff, "评论已关闭")
	}
	content, err = normalizeCommentContent(content)
	if err != nil {
		return nil, err
	}
	mount, err := s.replyMount(ctx, postID, replyToID)
	if err != nil {
		return nil, err
	}
	if err := s.CheckRate(ctx, addr, entity.ActionComment); err != nil {
		return nil, err
	}
	name, _, err := s.DisplayName(ctx, addr, nickname)
	if err != nil {
		return nil, err
	}
	c := &entity.Comment{
		PostID:      postID,
		AuthorAddr:  addr,
		AuthorName:  name,
		Content:     content,
		ParentID:    mount.ParentID,
		ReplyToID:   mount.ReplyToID,
		ReplyToAddr: mount.ReplyToAddr,
		ReplyToName: mount.ReplyToName,
		Status:      entity.StatusPending,
	}
	id, err := s.store.InsertComment(ctx, c)
	if err != nil {
		s.log.Error("insert comment failed", "post", postID, "addr", addr, "replyTo", replyToID, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	c.ID = id
	if err := s.applyModeration(ctx, entity.TargetComment, c.ID, content, nil, func(status int16) error {
		return s.commitCommentStatus(ctx, c, false, status, p)
	}); err != nil {
		c.Status = statusFromModErr(err)
		return c, err
	}
	c.Status = entity.StatusVisible
	return c, nil
}

func (s *Service) AdminCreateOfficialComment(ctx context.Context, postID int64, content, operator string, replyToID int64) (*entity.Comment, error) {
	content, err := normalizeCommentContent(content)
	if err != nil {
		return nil, err
	}
	if postID <= 0 {
		s.log.Error("official comment invalid post", "post", postID, "operator", operator)
		return nil, xerror.New(xerror.ParamInvalid, "参数错误")
	}
	p, err := s.store.GetPost(ctx, postID)
	if err != nil {
		s.log.Error("get post for official comment failed", "post", postID, "operator", operator, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if p == nil {
		s.log.Error("official comment post missing", "post", postID, "operator", operator)
		return nil, xerror.New(xerror.NotFound, "帖子不存在")
	}
	if p.Status == entity.StatusDeleted || p.Status == entity.StatusDraft {
		s.log.Error("official comment post status blocked", "post", postID, "status", p.Status, "operator", operator)
		return nil, xerror.New(xerror.ParamInvalid, "该帖不能回复")
	}
	mount, err := s.replyMount(ctx, postID, replyToID)
	if err != nil {
		s.log.Error("official comment reply target rejected", "post", postID, "replyTo", replyToID, "operator", operator, "err", err)
		return nil, err
	}
	oa, err := s.EnsurePlatformOfficial(ctx)
	if err != nil {
		s.log.Error("ensure platform official for comment failed", "post", postID, "operator", operator, "err", err)
		return nil, err
	}
	c := &entity.Comment{
		PostID:      postID,
		AuthorAddr:  entity.PlatformOfficialAddr,
		AuthorName:  oa.Name,
		Content:     content,
		ParentID:    mount.ParentID,
		ReplyToID:   mount.ReplyToID,
		ReplyToAddr: mount.ReplyToAddr,
		ReplyToName: mount.ReplyToName,
		Status:      entity.StatusVisible,
	}
	id, err := s.store.InsertComment(ctx, c)
	if err != nil {
		s.log.Error("insert official comment failed", "post", postID, "operator", operator, "replyTo", replyToID, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	c.ID = id
	if err := s.commitCommentStatus(ctx, c, false, entity.StatusVisible, p); err != nil {
		s.log.Error("commit official comment visibility failed", "post", postID, "comment", id, "operator", operator, "err", err)
		return nil, err
	}
	s.log.Info("admin official comment created", "post", postID, "comment", id, "operator", operator, "replyTo", replyToID, "name", oa.Name, "commentsOff", p.CommentsEnabled == 0, "runes", utf8.RuneCountInString(content))
	return c, nil
}

func (s *Service) replyMount(ctx context.Context, postID, replyToID int64) (ReplyMount, error) {
	if replyToID <= 0 {
		return ReplyMount{}, nil
	}
	target, err := s.store.GetComment(ctx, replyToID)
	if err != nil {
		s.log.Error("get reply target failed", "post", postID, "replyTo", replyToID, "err", err)
		return ReplyMount{}, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	mount, err := ResolveReplyMount(replyToID, postID, target)
	if err != nil {
		s.log.Error("reply target rejected", "post", postID, "replyTo", replyToID, "err", err)
		return ReplyMount{}, err
	}
	return mount, nil
}

func (s *Service) commitCommentStatus(ctx context.Context, c *entity.Comment, wasVisible bool, now int16, p *entity.Post) error {
	if c == nil {
		return xerror.New(xerror.NotFound, "评论不存在")
	}
	var err error
	if p == nil || p.ID != c.PostID {
		p, err = s.store.GetPost(ctx, c.PostID)
		if err != nil {
			s.log.Error("get post for comment visibility failed", "post", c.PostID, "comment", c.ID, "err", err)
			return xerror.Wrap(xerror.Internal, "内部错误", err)
		}
	}
	author := ""
	title := ""
	if p != nil {
		author = p.AuthorAddr
		title = PostTitleSnapshot(p.Title, p.Intro, p.Content)
	}
	recipient := ""
	if now == entity.StatusVisible && !wasVisible && p != nil && p.Status == entity.StatusVisible {
		recipient = NoticeRecipient(c.AuthorAddr, author, c.ReplyToAddr, c.ParentID)
	}
	if err := s.store.ApplyCommentVisibility(ctx, repository.CommentVisInput{
		Comment:    c,
		WasVisible: wasVisible,
		NowStatus:  now,
		PostAuthor: author,
		PostTitle:  title,
		Recipient:  recipient,
	}); err != nil {
		s.log.Error("apply comment visibility failed", "post", c.PostID, "comment", c.ID, "wasVisible", wasVisible, "status", now, "parent", c.ParentID, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	c.Status = now
	return nil
}

func normalizeCommentContent(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" || utf8.RuneCountInString(content) > maxCommentRunes {
		return "", xerror.New(xerror.ParamInvalid, "内容不合法")
	}
	return content, nil
}

func (s *Service) ListComments(ctx context.Context, postID int64, viewer string, page entity.PageQuery, thread bool) ([]entity.Comment, int64, int64, error) {
	p, err := s.requireVisiblePost(ctx, postID)
	if err != nil {
		return nil, 0, 0, err
	}
	_ = viewer
	if thread {
		list, total, err := s.store.ListThreadHeads(ctx, postID, page)
		if err != nil {
			s.log.Error("list thread comments failed", "post", postID, "err", err)
			return nil, 0, 0, xerror.Wrap(xerror.Internal, "内部错误", err)
		}
		for i := range list {
			redactThreadHead(&list[i])
			replies, err := s.store.ListVisibleReplyPreview(ctx, postID, list[i].ID, replyPreviewLimit)
			if err != nil {
				s.log.Error("list reply preview failed", "post", postID, "comment", list[i].ID, "err", err)
				return nil, 0, 0, xerror.Wrap(xerror.Internal, "内部错误", err)
			}
			if replies == nil {
				replies = []entity.Comment{}
			}
			list[i].Replies = replies
		}
		total, commentCount := commentListCounts(true, total, p.CommentCount)
		return list, total, commentCount, nil
	}
	list, total, err := s.store.ListComments(ctx, repository.CommentListFilter{PostID: postID, OnlyVisible: true}, page)
	if err != nil {
		s.log.Error("list comments failed", "post", postID, "err", err)
		return nil, 0, 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	total, commentCount := commentListCounts(false, total, p.CommentCount)
	return list, total, commentCount, nil
}

func (s *Service) ListReplies(ctx context.Context, postID, commentID int64, page entity.PageQuery) ([]entity.Comment, int64, error) {
	if _, err := s.requireVisiblePost(ctx, postID); err != nil {
		return nil, 0, err
	}
	root, err := s.store.GetComment(ctx, commentID)
	if err != nil {
		s.log.Error("get reply root failed", "post", postID, "comment", commentID, "err", err)
		return nil, 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if root == nil || root.PostID != postID || root.ParentID != 0 {
		return nil, 0, xerror.New(xerror.NotFound, "评论不存在")
	}
	list, total, err := s.store.ListComments(ctx, repository.CommentListFilter{PostID: postID, ReplyParentID: commentID, OnlyVisible: true}, page)
	if err != nil {
		s.log.Error("list replies failed", "post", postID, "comment", commentID, "err", err)
		return nil, 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	return list, total, nil
}

func redactThreadHead(c *entity.Comment) {
	if c == nil || c.Status == entity.StatusVisible {
		return
	}
	c.Deleted = true
	c.Content = ""
	c.AuthorName = ""
	c.AuthorAddr = ""
}

func (s *Service) DeleteOwnComment(ctx context.Context, id int64, addr string) error {
	c, err := s.store.GetComment(ctx, id)
	if err != nil {
		s.log.Error("get comment for delete failed", "id", id, "addr", addr, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if c == nil {
		return xerror.New(xerror.NotFound, "评论不存在")
	}
	if c.AuthorAddr != addr {
		return xerror.New(xerror.Forbidden, "无权操作")
	}
	wasVisible := c.Status == entity.StatusVisible
	if err := s.commitCommentStatus(ctx, c, wasVisible, entity.StatusDeleted, nil); err != nil {
		return err
	}
	return nil
}
