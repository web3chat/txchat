package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	"github.com/web3chat/txchat/dtalk/service/square/internal/moderator"
	"github.com/web3chat/txchat/dtalk/service/square/internal/repository"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

const maxPostRunes = 5000
const maxTitleRunes = 64
const maxIntroRunes = 200
const maxCommentRunes = 500
const maxImages = 9
const ownPostEditWindow = 20 * time.Minute

type CreatePostInput struct {
	Addr            string
	Nickname        string
	Title           string
	Intro           string
	Content         string
	MediaURLs       []string
	CommentsEnabled *bool
}

func (s *Service) CreatePost(ctx context.Context, in CreatePostInput) (*entity.Post, error) {
	if err := s.AssertSquarePostable(ctx, in.Addr); err != nil {
		return nil, err
	}
	if err := s.AssertNotBanned(ctx, in.Addr); err != nil {
		return nil, err
	}
	title, intro, content, err := normalizePostText(in.Title, in.Intro, in.Content, false)
	if err != nil {
		return nil, err
	}
	if len(in.MediaURLs) > maxImages {
		return nil, xerror.New(xerror.ParamInvalid, "图片过多")
	}
	if err := s.rejectNearDuplicate(ctx, in.Addr, title, intro, content); err != nil {
		return nil, err
	}
	if err := s.CheckRate(ctx, in.Addr, entity.ActionPost); err != nil {
		return nil, err
	}
	name, kind, err := s.DisplayName(ctx, in.Addr, in.Nickname)
	if err != nil {
		return nil, err
	}
	enabled := int16(1)
	if kind == entity.KindOfficial && in.CommentsEnabled != nil && !*in.CommentsEnabled {
		enabled = 0
	}
	media, _ := json.Marshal(in.MediaURLs)
	if media == nil {
		media = []byte("[]")
	}
	post := &entity.Post{
		AuthorAddr:      in.Addr,
		AuthorName:      name,
		AuthorKind:      kind,
		Title:           title,
		Intro:           intro,
		Content:         content,
		MediaJSON:       media,
		CommentsEnabled: enabled,
		Status:          entity.StatusPending,
	}
	id, err := s.store.InsertPost(ctx, post)
	if err != nil {
		s.log.Error("insert post failed", "addr", in.Addr, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	post.ID = id
	s.fillAuthorAvatar(ctx, post)
	if err := s.applyModeration(ctx, entity.TargetPost, post.ID, moderationText(title, intro, content), in.MediaURLs, func(status int16) error {
		return s.store.UpdatePostStatus(ctx, post.ID, status)
	}); err != nil {
		post.Status = statusFromModErr(err)
		return post, err
	}
	post.Status = entity.StatusVisible
	return post, nil
}

type AdminCreatePostInput struct {
	Title           string
	Intro           string
	Content         string
	MediaURLs       []string
	CommentsEnabled *bool
	Name            string
	Avatar          string
	Draft           bool
	Tag             string
	PublishAt       *time.Time
	ExpireAfterSec  int64
}

func compactMediaURLs(urls []string) ([]string, error) {
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		out = append(out, u)
	}
	if len(out) > maxImages {
		return nil, xerror.New(xerror.ParamInvalid, "图片过多")
	}
	return out, nil
}

// NormalizeOfficialTag 校验并规范化官方帖标签；空字符串表示未分类。
func NormalizeOfficialTag(tag string) (string, error) {
	tag = strings.TrimSpace(tag)
	if tag == "新闻" {
		tag = entity.OfficialPostTagMessage
	}
	if tag == "" {
		return "", nil
	}
	for _, allowed := range entity.OfficialPostTags() {
		if tag == allowed {
			return tag, nil
		}
	}
	return "", xerror.New(xerror.ParamInvalid, "标签无效，可选：公告、消息、活动")
}

func normalizePostText(title, intro, content string, requireTitle bool) (string, string, string, error) {
	title = strings.TrimSpace(title)
	intro = strings.TrimSpace(intro)
	content = strings.TrimSpace(content)
	if requireTitle && title == "" {
		return "", "", "", xerror.New(xerror.ParamInvalid, "请填写标题")
	}
	if utf8.RuneCountInString(title) > maxTitleRunes {
		return "", "", "", xerror.New(xerror.ParamInvalid, "标题过长")
	}
	if utf8.RuneCountInString(intro) > maxIntroRunes {
		return "", "", "", xerror.New(xerror.ParamInvalid, "简介过长")
	}
	if utf8.RuneCountInString(content) > maxPostRunes {
		return "", "", "", xerror.New(xerror.ParamInvalid, "内容不合法")
	}
	if requireTitle && content == "" {
		return "", "", "", xerror.New(xerror.ParamInvalid, "内容不合法")
	}
	return title, intro, content, nil
}

// 普通草稿至少要有标题或正文。定时和正式发布另有更严的校验。
func draftNeedsText(status int16, scheduled bool, title, content string) error {
	if status == entity.StatusDraft && !scheduled && title == "" && content == "" {
		return xerror.New(xerror.ParamInvalid, "请填写标题或正文")
	}
	return nil
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func officialPostStatus(draft bool) int16 {
	if draft {
		return entity.StatusDraft
	}
	return entity.StatusVisible
}

func officialEditable(status int16) bool {
	return status == entity.StatusDraft || status == entity.StatusVisible || status == entity.StatusHidden
}

func commentsEnabledFlag(v *bool) int16 {
	if v != nil && !*v {
		return 0
	}
	return 1
}

func moderationText(title, intro, content string) string {
	parts := make([]string, 0, 3)
	if title != "" {
		parts = append(parts, title)
	}
	if intro != "" {
		parts = append(parts, intro)
	}
	parts = append(parts, content)
	return strings.Join(parts, "\n")
}

func (s *Service) EnsurePlatformOfficial(ctx context.Context) (*entity.OfficialAccount, error) {
	oa, err := s.store.GetOfficial(ctx, entity.PlatformOfficialAddr)
	if err != nil {
		s.log.Error("get platform official failed", "addr", entity.PlatformOfficialAddr, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if oa != nil {
		return oa, nil
	}
	oa = &entity.OfficialAccount{
		Address: entity.PlatformOfficialAddr,
		Name:    entity.PlatformOfficialName,
		Status:  entity.OAStatusNormal,
	}
	if err := s.store.UpsertOfficial(ctx, oa); err != nil {
		s.log.Error("upsert platform official failed", "addr", entity.PlatformOfficialAddr, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.log.Info("platform official created", "addr", entity.PlatformOfficialAddr, "name", oa.Name)
	return oa, nil
}

func (s *Service) PlatformOfficial(ctx context.Context) (*entity.OfficialAccount, error) {
	return s.EnsurePlatformOfficial(ctx)
}

func (s *Service) UpdatePlatformOfficial(ctx context.Context, name, avatar string) (*entity.OfficialAccount, error) {
	oa, err := s.EnsurePlatformOfficial(ctx)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = oa.Name
	}
	avatar = strings.TrimSpace(avatar)
	if avatar == "" {
		avatar = oa.Avatar
	}
	if name != "" && name != oa.Name {
		s.log.Info("ignore platform official rename on profile save", "kept", oa.Name, "ignored", name)
	}
	if err := s.store.UpdateOfficialAvatar(ctx, entity.PlatformOfficialAddr, avatar); err != nil {
		s.log.Error("update platform official avatar failed", "avatar_set", avatar != "", "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	oa, err = s.store.GetOfficial(ctx, entity.PlatformOfficialAddr)
	if err != nil {
		s.log.Error("reload platform official after update failed", "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if oa == nil {
		s.log.Error("platform official missing after update")
		return nil, xerror.New(xerror.Internal, "内部错误")
	}
	s.log.Info("platform official profile saved", "name", oa.Name, "avatar_set", strings.TrimSpace(oa.Avatar) != "")
	return oa, nil
}

func (s *Service) AdminCreatePost(ctx context.Context, in AdminCreatePostInput) (*entity.Post, error) {
	timing, err := ResolveOfficialTiming(time.Now(), false, in.Draft, in.PublishAt, in.ExpireAfterSec)
	if err != nil {
		s.log.Warn("reject official post timing", "draft", in.Draft, "publishAt", formatTimePtr(in.PublishAt), "expireAfterSec", in.ExpireAfterSec, "err", err)
		return nil, err
	}
	title, intro, content, err := normalizePostText(in.Title, in.Intro, in.Content, timing.Status != entity.StatusDraft || timing.Scheduled)
	if err != nil {
		return nil, err
	}
	if err = draftNeedsText(timing.Status, timing.Scheduled, title, content); err != nil {
		s.log.Warn("reject empty official draft", "draft", in.Draft, "scheduled", timing.Scheduled, "err", err)
		return nil, err
	}
	mediaURLs, err := compactMediaURLs(in.MediaURLs)
	if err != nil {
		return nil, err
	}
	tag, err := NormalizeOfficialTag(in.Tag)
	if err != nil {
		return nil, err
	}
	persona, oa, err := s.resolveOfficialPersona(ctx, in.Name)
	if err != nil {
		return nil, err
	}
	enabled := commentsEnabledFlag(in.CommentsEnabled)
	media, _ := json.Marshal(mediaURLs)
	if media == nil {
		media = []byte("[]")
	}
	post := &entity.Post{
		AuthorAddr:      entity.PlatformOfficialAddr,
		AuthorName:      persona,
		AuthorKind:      entity.KindOfficial,
		AuthorAvatar:    oa.Avatar,
		Title:           title,
		Intro:           intro,
		Content:         content,
		Tag:             tag,
		MediaJSON:       media,
		CommentsEnabled: enabled,
		Status:          timing.Status,
		PublishAt:       timing.PublishAt,
		ExpireAt:        timing.ExpireAt,
		ExpireAfterSec:  timing.ExpireAfterSec,
	}
	id, err := s.store.InsertPost(ctx, post)
	if err != nil {
		s.log.Error("admin insert official post failed", "name", persona, "draft", in.Draft, "scheduled", timing.Scheduled, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	post.ID = id
	s.fillAuthorAvatar(ctx, post)
	s.log.Info("admin official post created", "id", id, "name", persona, "title", title, "tag", tag, "images", len(mediaURLs), "draft", in.Draft, "scheduled", timing.Scheduled, "publishAt", formatTimePtr(timing.PublishAt), "expireAfterSec", timing.ExpireAfterSec, "status", post.Status, "commentsEnabled", enabled == 1)
	return post, nil
}

func (s *Service) AdminUpdateOfficialPost(ctx context.Context, id int64, in AdminCreatePostInput) (*entity.Post, error) {
	if id <= 0 {
		return nil, xerror.New(xerror.ParamInvalid, "参数错误")
	}
	existing, err := s.store.GetPost(ctx, id)
	if err != nil {
		s.log.Error("get official post for update failed", "id", id, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if existing == nil {
		s.log.Error("official post missing for update", "id", id)
		return nil, xerror.New(xerror.NotFound, "帖子不存在")
	}
	if existing.AuthorAddr != entity.PlatformOfficialAddr {
		s.log.Error("official post author mismatch", "id", id, "author", existing.AuthorAddr)
		return nil, xerror.New(xerror.Forbidden, "无权操作")
	}
	if !officialEditable(existing.Status) {
		s.log.Error("official post status not editable", "id", id, "status", existing.Status)
		return nil, xerror.New(xerror.ParamInvalid, "该帖不能编辑")
	}
	if existing.Status != entity.StatusDraft && in.Draft {
		s.log.Error("official published post cannot save as draft", "id", id, "status", existing.Status)
		return nil, xerror.New(xerror.ParamInvalid, "已发布帖不能存为草稿")
	}
	live := existing.Status != entity.StatusDraft
	timing, err := ResolveOfficialTiming(time.Now(), live, in.Draft, in.PublishAt, in.ExpireAfterSec)
	if err != nil {
		s.log.Warn("reject official post timing", "id", id, "status", existing.Status, "draft", in.Draft, "publishAt", formatTimePtr(in.PublishAt), "expireAfterSec", in.ExpireAfterSec, "err", err)
		return nil, err
	}
	title, intro, content, err := normalizePostText(in.Title, in.Intro, in.Content, timing.Status != entity.StatusDraft || timing.Scheduled)
	if err != nil {
		return nil, err
	}
	if err = draftNeedsText(timing.Status, timing.Scheduled, title, content); err != nil {
		s.log.Warn("reject empty official draft", "id", id, "draft", in.Draft, "scheduled", timing.Scheduled, "err", err)
		return nil, err
	}
	mediaURLs, err := compactMediaURLs(in.MediaURLs)
	if err != nil {
		return nil, err
	}
	tag, err := NormalizeOfficialTag(in.Tag)
	if err != nil {
		return nil, err
	}
	enabled := commentsEnabledFlag(in.CommentsEnabled)
	media, _ := json.Marshal(mediaURLs)
	if media == nil {
		media = []byte("[]")
	}
	post := &entity.Post{
		ID:              id,
		AuthorName:      existing.AuthorName,
		Title:           title,
		Intro:           intro,
		Content:         content,
		Tag:             tag,
		MediaJSON:       media,
		CommentsEnabled: enabled,
		Status:          timing.Status,
		PublishAt:       timing.PublishAt,
		ExpireAt:        timing.ExpireAt,
		ExpireAfterSec:  timing.ExpireAfterSec,
	}
	ok, err := s.store.UpdateOfficialPost(ctx, post, existing.Status)
	if err != nil {
		s.log.Error("admin update official post failed", "id", id, "fromStatus", existing.Status, "draft", in.Draft, "scheduled", timing.Scheduled, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if !ok {
		s.log.Error("admin update official post missed", "id", id, "fromStatus", existing.Status, "draft", in.Draft)
		return nil, xerror.New(xerror.NotFound, "帖子不存在或状态已变")
	}
	fresh, err := s.store.GetPost(ctx, id)
	if err != nil {
		s.log.Error("reload official post after update failed", "id", id, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if fresh == nil {
		s.log.Error("official post missing after update", "id", id)
		return nil, xerror.New(xerror.Internal, "内部错误")
	}
	s.fillAuthorAvatar(ctx, fresh)
	s.log.Info("admin official post updated", "id", id, "name", existing.AuthorName, "title", title, "tag", tag, "images", len(mediaURLs), "draft", in.Draft, "scheduled", timing.Scheduled, "publishAt", formatTimePtr(timing.PublishAt), "expireAfterSec", timing.ExpireAfterSec, "status", fresh.Status, "commentsEnabled", enabled == 1)
	return fresh, nil
}

func (s *Service) GetPost(ctx context.Context, id int64, viewer string, countView bool) (*entity.Post, error) {
	if err := s.AssertSquareReadable(ctx); err != nil {
		return nil, err
	}
	p, err := s.store.GetPost(ctx, id)
	if err != nil {
		s.log.Error("get post failed", "id", id, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if p == nil {
		return nil, xerror.New(xerror.NotFound, "帖子不存在")
	}
	if p.Status != entity.StatusVisible && p.AuthorAddr != viewer {
		return nil, xerror.New(xerror.NotFound, "帖子不存在")
	}
	if countView && p.Status == entity.StatusVisible && viewer != "" && viewer != p.AuthorAddr {
		loc, _ := timeLocation()
		day := nowIn(loc)
		if _, err := s.store.AddView(ctx, p.ID, viewer, day); err != nil {
			s.log.Error("add view failed", "id", id, "addr", viewer, "err", err)
		} else {
			fresh, _ := s.store.GetPost(ctx, id)
			if fresh != nil {
				p = fresh
			}
		}
	}
	liked, err := s.store.LikedSet(ctx, viewer, []int64{p.ID})
	if err == nil {
		_, p.Liked = liked[p.ID]
	}
	s.fillAuthorAvatar(ctx, p)
	return p, nil
}

func (s *Service) ListFeed(ctx context.Context, viewer string, f repository.PostListFilter, page entity.PageQuery) ([]entity.Post, int64, error) {
	if err := s.AssertSquareReadable(ctx); err != nil {
		return nil, 0, err
	}
	f.OnlyVisible = true
	switch strings.ToLower(strings.TrimSpace(f.Feed)) {
	case "", entity.FeedLatest:
	case entity.FeedHot:
		f.Order = "hot"
		if f.From == nil {
			since := time.Now().Add(-time.Duration(s.cfg.HotWindowDays) * 24 * time.Hour)
			f.From = &since
		}
	case entity.FeedFollowing:
		f.FollowerAddr = viewer
	default:
		return nil, 0, xerror.New(xerror.ParamInvalid, "feed 不合法")
	}
	page = page.Normalize()
	pins, err := s.store.ListPinned(ctx, f)
	if err != nil {
		s.log.Error("list pinned feed failed", "feed", f.Feed, "addrs", f.AuthorAddrs, "names", f.AuthorNames, "kind", f.Kind, "tags", f.Tags, "follower", f.FollowerAddr, "err", err)
		return nil, 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	f.ExcludePinned = true
	list, total, err := s.store.ListPosts(ctx, f, page)
	if err != nil {
		s.log.Error("list feed failed", "feed", f.Feed, "addrs", f.AuthorAddrs, "names", f.AuthorNames, "tags", f.Tags, "order", f.Order, "follower", f.FollowerAddr, "page", page.Page, "pageSize", page.PageSize, "err", err)
		return nil, 0, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	list = mergePinnedPosts(pins, list, page.Page)
	total += int64(len(pins))
	s.fillLiked(ctx, viewer, list)
	s.fillAuthorAvatars(ctx, list)
	return list, total, nil
}

func mergePinnedPosts(pins, list []entity.Post, page int) []entity.Post {
	if page > 1 || len(pins) == 0 {
		return list
	}
	seen := make(map[int64]struct{}, len(pins))
	out := make([]entity.Post, 0, len(pins)+len(list))
	for _, p := range pins {
		seen[p.ID] = struct{}{}
		out = append(out, p)
	}
	for _, p := range list {
		if _, ok := seen[p.ID]; ok {
			continue
		}
		out = append(out, p)
	}
	return out
}

func (s *Service) ListMyPosts(ctx context.Context, addr string, page entity.PageQuery) ([]entity.Post, int64, entity.AuthorTraffic, error) {
	if err := s.AssertSquareReadable(ctx); err != nil {
		return nil, 0, entity.AuthorTraffic{}, err
	}
	stats, err := s.store.AuthorTraffic(ctx, addr)
	if err != nil {
		s.log.Error("author traffic failed", "addr", addr, "err", err)
		return nil, 0, entity.AuthorTraffic{}, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	list, total, err := s.store.ListPosts(ctx, repository.PostListFilter{
		AuthorAddrs:    []string{addr},
		ExcludeDeleted: true,
		ExcludeDraft:   true,
	}, page)
	if err != nil {
		s.log.Error("list my posts failed", "addr", addr, "page", page.Page, "err", err)
		return nil, 0, entity.AuthorTraffic{}, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.fillLiked(ctx, addr, list)
	s.fillAuthorAvatars(ctx, list)
	return list, total, stats, nil
}

func (s *Service) UpdateOwnPost(ctx context.Context, id int64, in CreatePostInput) (*entity.Post, error) {
	if id <= 0 {
		return nil, xerror.New(xerror.ParamInvalid, "参数错误")
	}
	if err := s.AssertSquarePostable(ctx, in.Addr); err != nil {
		return nil, err
	}
	if err := s.AssertNotBanned(ctx, in.Addr); err != nil {
		return nil, err
	}
	existing, err := s.store.GetPost(ctx, id)
	if err != nil {
		s.log.Error("get post for edit failed", "id", id, "addr", in.Addr, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if existing == nil || existing.Status == entity.StatusDeleted {
		s.log.Error("post missing for edit", "id", id, "addr", in.Addr)
		return nil, xerror.New(xerror.NotFound, "帖子不存在")
	}
	if existing.AuthorAddr != in.Addr {
		s.log.Error("edit post author mismatch", "id", id, "addr", in.Addr, "author", existing.AuthorAddr)
		return nil, xerror.New(xerror.Forbidden, "无权操作")
	}
	if existing.Status == entity.StatusDraft {
		s.log.Error("c-end cannot edit draft", "id", id, "addr", in.Addr)
		return nil, xerror.New(xerror.NotFound, "帖子不存在")
	}
	if err := assertOwnPostEditWindow(existing, time.Now()); err != nil {
		until := PostEditUntil(existing)
		untilAt := ""
		if until != nil {
			untilAt = until.Format(time.RFC3339)
		}
		s.log.Error("own post edit window closed", "id", id, "addr", in.Addr, "kind", existing.AuthorKind, "createdAt", existing.CreatedAt.Format(time.RFC3339), "until", untilAt)
		return nil, err
	}
	title, intro, content, err := normalizePostText(in.Title, in.Intro, in.Content, false)
	if err != nil {
		return nil, err
	}
	if content == "" {
		return nil, xerror.New(xerror.ParamInvalid, "内容不合法")
	}
	if len(in.MediaURLs) > maxImages {
		return nil, xerror.New(xerror.ParamInvalid, "图片过多")
	}
	if err := s.CheckRate(ctx, in.Addr, entity.ActionEdit); err != nil {
		return nil, err
	}
	name, kind, err := s.DisplayName(ctx, in.Addr, in.Nickname)
	if err != nil {
		return nil, err
	}
	enabled := existing.CommentsEnabled
	if kind == entity.KindOfficial && in.CommentsEnabled != nil {
		if *in.CommentsEnabled {
			enabled = 1
		} else {
			enabled = 0
		}
	}
	media, _ := json.Marshal(in.MediaURLs)
	if media == nil {
		media = []byte("[]")
	}
	post := &entity.Post{
		ID:              id,
		AuthorAddr:      in.Addr,
		AuthorName:      name,
		Title:           title,
		Intro:           intro,
		Content:         content,
		MediaJSON:       media,
		CommentsEnabled: enabled,
		Status:          entity.StatusPending,
	}
	ok, err := s.store.UpdateUserPost(ctx, post)
	if err != nil {
		s.log.Error("update own post failed", "id", id, "addr", in.Addr, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if !ok {
		s.log.Error("update own post missed", "id", id, "addr", in.Addr, "status", existing.Status)
		return nil, xerror.New(xerror.NotFound, "帖子不存在或不可编辑")
	}
	if err := s.applyModeration(ctx, entity.TargetPost, id, moderationText(title, intro, content), in.MediaURLs, func(status int16) error {
		return s.store.UpdatePostStatus(ctx, id, status)
	}); err != nil {
		fresh, loadErr := s.store.GetPost(ctx, id)
		if loadErr != nil {
			s.log.Error("reload post after edit moderation failed", "id", id, "addr", in.Addr, "err", loadErr)
		}
		if fresh != nil {
			s.fillAuthorAvatar(ctx, fresh)
			return fresh, err
		}
		return post, err
	}
	fresh, err := s.store.GetPost(ctx, id)
	if err != nil {
		s.log.Error("reload post after edit failed", "id", id, "addr", in.Addr, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if fresh == nil {
		s.log.Error("post missing after edit", "id", id, "addr", in.Addr)
		return nil, xerror.New(xerror.Internal, "内部错误")
	}
	liked, _ := s.store.LikedSet(ctx, in.Addr, []int64{id})
	_, fresh.Liked = liked[id]
	s.fillAuthorAvatar(ctx, fresh)
	s.log.Info("own post updated", "id", id, "addr", in.Addr, "status", fresh.Status, "title", title)
	return fresh, nil
}

func (s *Service) AdminPinPost(ctx context.Context, id int64) (*entity.Post, error) {
	p, err := s.store.GetPost(ctx, id)
	if err != nil {
		s.log.Error("get post for pin failed", "id", id, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if p == nil {
		s.log.Error("post missing for pin", "id", id)
		return nil, xerror.New(xerror.NotFound, "帖子不存在")
	}
	if p.Status != entity.StatusVisible {
		s.log.Error("pin rejected: not visible", "id", id, "status", p.Status)
		return nil, xerror.New(xerror.ParamInvalid, "只能置顶可见帖")
	}
	if p.Pinned() {
		s.fillAuthorAvatar(ctx, p)
		return p, nil
	}
	n, err := s.store.CountPinnedVisible(ctx)
	if err != nil {
		s.log.Error("count pinned failed", "id", id, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if n >= int64(entity.MaxPinnedPosts) {
		s.log.Error("pin limit reached", "id", id, "count", n, "limit", entity.MaxPinnedPosts)
		return nil, xerror.New(xerror.PinLimit, "最多置顶 3 条")
	}
	ok, err := s.store.SetPostPinned(ctx, id, true)
	if err != nil {
		s.log.Error("pin post failed", "id", id, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if !ok {
		s.log.Error("pin post missed", "id", id, "status", p.Status)
		return nil, xerror.New(xerror.ParamInvalid, "只能置顶可见帖")
	}
	fresh, err := s.store.GetPost(ctx, id)
	if err != nil {
		s.log.Error("reload post after pin failed", "id", id, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if fresh == nil {
		s.log.Error("post missing after pin", "id", id)
		return nil, xerror.New(xerror.Internal, "内部错误")
	}
	s.fillAuthorAvatar(ctx, fresh)
	s.log.Info("post pinned", "id", id, "author", fresh.AuthorAddr)
	return fresh, nil
}

func (s *Service) AdminUnpinPost(ctx context.Context, id int64) (*entity.Post, error) {
	p, err := s.store.GetPost(ctx, id)
	if err != nil {
		s.log.Error("get post for unpin failed", "id", id, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if p == nil {
		s.log.Error("post missing for unpin", "id", id)
		return nil, xerror.New(xerror.NotFound, "帖子不存在")
	}
	if !p.Pinned() {
		s.fillAuthorAvatar(ctx, p)
		return p, nil
	}
	if _, err := s.store.SetPostPinned(ctx, id, false); err != nil {
		s.log.Error("unpin post failed", "id", id, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	fresh, err := s.store.GetPost(ctx, id)
	if err != nil {
		s.log.Error("reload post after unpin failed", "id", id, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if fresh == nil {
		s.log.Error("post missing after unpin", "id", id)
		return nil, xerror.New(xerror.Internal, "内部错误")
	}
	s.fillAuthorAvatar(ctx, fresh)
	s.log.Info("post unpinned", "id", id, "author", fresh.AuthorAddr)
	return fresh, nil
}

func (s *Service) DeleteOwnPost(ctx context.Context, id int64, addr string) error {
	p, err := s.store.GetPost(ctx, id)
	if err != nil {
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if p == nil {
		return xerror.New(xerror.NotFound, "帖子不存在")
	}
	if p.AuthorAddr != addr {
		return xerror.New(xerror.Forbidden, "无权操作")
	}
	return s.store.UpdatePostStatus(ctx, id, entity.StatusDeleted)
}

func (s *Service) ToggleComments(ctx context.Context, id int64, addr string, enabled bool) error {
	if err := s.AssertSquarePostable(ctx, addr); err != nil {
		return err
	}
	if err := s.AssertNotBanned(ctx, addr); err != nil {
		return err
	}
	p, err := s.store.GetPost(ctx, id)
	if err != nil {
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if p == nil {
		return xerror.New(xerror.NotFound, "帖子不存在")
	}
	if p.AuthorAddr != addr || p.AuthorKind != entity.KindOfficial {
		return xerror.New(xerror.Forbidden, "仅公众号作者可开关评论")
	}
	v := int16(1)
	if !enabled {
		v = 0
	}
	return s.store.UpdateCommentsEnabled(ctx, id, v)
}

func (s *Service) Like(ctx context.Context, id int64, addr string) (*entity.Post, error) {
	if err := s.AssertNotBanned(ctx, addr); err != nil {
		return nil, err
	}
	p, err := s.requireVisiblePost(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.CheckRate(ctx, addr, entity.ActionLike); err != nil {
		return nil, err
	}
	if _, err := s.store.AddLike(ctx, id, addr); err != nil {
		s.log.Error("like failed", "id", id, "addr", addr, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	p, _ = s.store.GetPost(ctx, id)
	if p == nil {
		return nil, xerror.New(xerror.NotFound, "帖子不存在")
	}
	liked, _ := s.store.LikedSet(ctx, addr, []int64{id})
	_, p.Liked = liked[id]
	return p, nil
}

func (s *Service) Unlike(ctx context.Context, id int64, addr string) (*entity.Post, error) {
	p, err := s.requireVisiblePost(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := s.store.RemoveLike(ctx, id, addr); err != nil {
		s.log.Error("unlike failed", "id", id, "addr", addr, "err", err)
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	p, _ = s.store.GetPost(ctx, id)
	if p == nil {
		return nil, xerror.New(xerror.NotFound, "帖子不存在")
	}
	liked, _ := s.store.LikedSet(ctx, addr, []int64{id})
	_, p.Liked = liked[id]
	return p, nil
}

func (s *Service) requireVisiblePost(ctx context.Context, id int64) (*entity.Post, error) {
	if err := s.AssertSquareReadable(ctx); err != nil {
		return nil, err
	}
	p, err := s.store.GetPost(ctx, id)
	if err != nil {
		return nil, xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if p == nil || p.Status != entity.StatusVisible {
		return nil, xerror.New(xerror.NotFound, "帖子不存在")
	}
	return p, nil
}

func (s *Service) fillLiked(ctx context.Context, viewer string, list []entity.Post) {
	ids := make([]int64, 0, len(list))
	for i := range list {
		ids = append(ids, list[i].ID)
	}
	set, err := s.store.LikedSet(ctx, viewer, ids)
	if err != nil {
		s.log.Error("liked set failed", "addr", viewer, "err", err)
		return
	}
	for i := range list {
		_, list[i].Liked = set[list[i].ID]
	}
}

func (s *Service) fillAuthorAvatar(ctx context.Context, p *entity.Post) {
	if p == nil {
		return
	}
	list := []entity.Post{*p}
	s.fillAuthorAvatars(ctx, list)
	p.AuthorAvatar = list[0].AuthorAvatar
	p.AuthorIntro = list[0].AuthorIntro
	p.AuthorGold = list[0].AuthorGold
}

func (s *Service) fillAuthorAvatars(ctx context.Context, list []entity.Post) {
	addrs := make([]string, 0)
	for i := range list {
		if list[i].AuthorKind == entity.KindOfficial {
			addrs = append(addrs, list[i].AuthorAddr)
		}
	}
	if len(addrs) == 0 {
		return
	}
	m, err := s.store.OfficialProfileMap(ctx, addrs)
	if err != nil {
		s.log.Error("official profile map failed", "addrs", addrs, "err", err)
		return
	}
	personas, err := s.store.OfficialNameAvatarMap(ctx)
	if err != nil {
		s.log.Error("official name avatar map failed", "err", err)
		personas = map[string]repository.OfficialPersonaFace{}
	}
	for i := range list {
		if list[i].AuthorKind != entity.KindOfficial {
			continue
		}
		meta := m[list[i].AuthorAddr]
		list[i].AuthorGold = meta.Gold
		list[i].AuthorAvatar = meta.Avatar
		if list[i].AuthorAddr != entity.PlatformOfficialAddr {
			continue
		}
		face, ok := personas[list[i].AuthorName]
		if ok {
			list[i].AuthorAvatar = face.Avatar
			list[i].AuthorIntro = face.Intro
		}
	}
}

func timeLocation() (*time.Location, error) {
	return time.LoadLocation("Asia/Shanghai")
}

func ownPostHasEditDeadline(p *entity.Post) bool {
	if p == nil {
		return false
	}
	return p.AuthorAddr != entity.PlatformOfficialAddr
}

// PostEditUntil 个人帖与公众号帖的可编辑截止时间（发布时间起 20 分钟）。运营台官方账号帖不限时，返回 nil。
func PostEditUntil(p *entity.Post) *time.Time {
	if !ownPostHasEditDeadline(p) {
		return nil
	}
	until := p.CreatedAt.Add(ownPostEditWindow)
	return &until
}

func assertOwnPostEditWindow(p *entity.Post, now time.Time) error {
	until := PostEditUntil(p)
	if until == nil {
		return nil
	}
	if now.Before(*until) || now.Equal(*until) {
		return nil
	}
	return xerror.New(xerror.EditWindowClosed, "超过可编辑时间")
}

func nowIn(loc *time.Location) time.Time {
	if loc == nil {
		return time.Now()
	}
	return time.Now().In(loc)
}

func IsRankOrder(order string) bool {
	switch strings.ToLower(strings.TrimSpace(order)) {
	case "hot", "likes", "comments":
		return true
	default:
		return false
	}
}

func IsAuthorRankOrder(order string) bool {
	switch strings.ToLower(strings.TrimSpace(order)) {
	case "likes", "views", "view":
		return true
	default:
		return false
	}
}

// RankWindowStart 返回上海时区自然日窗口起点：当日 0 点、含今日共 7 天、含今日共 30 天。
func RankWindowStart(window string, now time.Time) *time.Time {
	loc, err := timeLocation()
	if err != nil || loc == nil {
		loc = time.Local
	}
	now = now.In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	start := today
	switch strings.ToLower(strings.TrimSpace(window)) {
	case "today", "day", "1":
		start = today
	case "month", "30":
		start = today.AddDate(0, 0, -29)
	default:
		start = today.AddDate(0, 0, -6)
	}
	return &start
}

func (s *Service) applyModeration(ctx context.Context, targetType string, targetID int64, text string, images []string, setStatus func(int16) error) error {
	if n := len(images); n > 0 {
		s.log.Info("skip image moderation", "type", targetType, "id", targetID, "images", n)
	}
	res, latency, err := s.mod.Moderate(ctx, text, nil)
	if err != nil {
		s.log.Error("moderate failed", "type", targetType, "id", targetID, "err", err)
		res = moderator.Result{Verdict: entity.VerdictReview, Reason: "moderator error", Unavailable: true}
	}
	level := s.moderationLevel(ctx)
	before := res.Verdict
	if level == entity.ModerationStrict {
		res = moderator.TightenResult(res)
	} else {
		res = moderator.LoosenResult(res)
	}
	if before != res.Verdict {
		s.log.Info("moderation policy applied", "type", targetType, "id", targetID, "level", level, "from", before, "to", res.Verdict)
	}
	cats := moderator.CategoriesJSON(res.Categories)
	m := &entity.Moderation{
		TargetType: targetType,
		TargetID:   targetID,
		Verdict:    res.Verdict,
		Categories: cats,
		Reason:     res.Reason,
		Model:      s.cfg.DeepSeek.Model,
		LatencyMS:  latency,
	}
	if err := s.store.InsertModeration(ctx, m); err != nil {
		s.log.Error("insert moderation failed", "type", targetType, "id", targetID, "err", err)
	}
	st := moderator.StatusOf(res.Verdict)
	if err := setStatus(st); err != nil {
		s.log.Error("set status after moderation failed", "type", targetType, "id", targetID, "status", st, "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	s.log.Info("moderation done", "type", targetType, "id", targetID, "level", level, "verdict", res.Verdict, "categories", res.Categories, "images", len(images), "latency_ms", latency)
	switch res.Verdict {
	case entity.VerdictReject:
		return xerror.New(xerror.ContentIllegal, "内容不合规，无法发布")
	case entity.VerdictReview:
		return xerror.New(xerror.PendingReview, "已提交，等待审核")
	default:
		return nil
	}
}

func statusFromModErr(err error) int16 {
	if ae, ok := xerror.AsApp(err); ok {
		if ae.Code == xerror.ContentIllegal {
			return entity.StatusAIReject
		}
		if ae.Code == xerror.PendingReview {
			return entity.StatusPending
		}
	}
	return entity.StatusPending
}
