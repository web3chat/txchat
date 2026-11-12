package app

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	"github.com/web3chat/txchat/dtalk/service/square/internal/middleware"
	"github.com/web3chat/txchat/dtalk/service/square/internal/pkg/httpresp"
	"github.com/web3chat/txchat/dtalk/service/square/internal/repository"
	"github.com/web3chat/txchat/dtalk/service/square/internal/usecase"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

type Handler struct {
	svc *usecase.Service
}

func New(svc *usecase.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) SquareStatus(c *gin.Context) {
	httpresp.AppOK(c, h.svc.SquareStatus(c.Request.Context(), middleware.Address(c)))
}

type createPostReq struct {
	Title           string   `json:"title"`
	Intro           string   `json:"intro"`
	Content         string   `json:"content"`
	Nickname        string   `json:"nickname"`
	MediaURLs       []string `json:"mediaUrls"`
	CommentsEnabled *bool    `json:"commentsEnabled"`
}

func (h *Handler) CreatePost(c *gin.Context) {
	var req createPostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	p, err := h.svc.CreatePost(c.Request.Context(), usecase.CreatePostInput{
		Addr:            middleware.Address(c),
		Nickname:        req.Nickname,
		Title:           req.Title,
		Intro:           req.Intro,
		Content:         req.Content,
		MediaURLs:       req.MediaURLs,
		CommentsEnabled: req.CommentsEnabled,
	})
	if err != nil {
		httpresp.AppErr(c, err)
		return
	}
	httpresp.AppOK(c, postJSON(p))
}

func (h *Handler) ListPosts(c *gin.Context) {
	page := parsePage(c)
	f := repository.PostListFilter{
		AuthorAddrs: parseTrimmed(c.QueryArray("authorAddr")),
		AuthorNames: parseTrimmed(c.QueryArray("authorName")),
		Feed:        strings.ToLower(strings.TrimSpace(c.Query("feed"))),
	}
	if v := c.Query("kind"); v != "" {
		n, _ := strconv.ParseInt(v, 10, 16)
		k := int16(n)
		f.Kind = &k
	}
	f.Tags = parseTrimmed(c.QueryArray("tag"))
	list, total, err := h.svc.ListFeed(c.Request.Context(), middleware.Address(c), f, page)
	if err != nil {
		httpresp.AppErr(c, err)
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, postJSON(&list[i]))
	}
	httpresp.AppOK(c, gin.H{"list": items, "total": total, "page": page.Page, "pageSize": page.PageSize})
}

func (h *Handler) GetPost(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	p, err := h.svc.GetPost(c.Request.Context(), id, middleware.Address(c), true)
	if err != nil {
		httpresp.AppErr(c, err)
		return
	}
	httpresp.AppOK(c, postJSON(p))
}

func (h *Handler) UpdatePost(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	var req createPostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	p, err := h.svc.UpdateOwnPost(c.Request.Context(), id, usecase.CreatePostInput{
		Addr:            middleware.Address(c),
		Nickname:        req.Nickname,
		Title:           req.Title,
		Intro:           req.Intro,
		Content:         req.Content,
		MediaURLs:       req.MediaURLs,
		CommentsEnabled: req.CommentsEnabled,
	})
	if err != nil {
		httpresp.AppErr(c, err)
		return
	}
	httpresp.AppOK(c, postJSON(p))
}

func (h *Handler) DeletePost(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	if err := h.svc.DeleteOwnPost(c.Request.Context(), id, middleware.Address(c)); err != nil {
		httpresp.AppErr(c, err)
		return
	}
	httpresp.AppOK(c, gin.H{"id": id})
}

func (h *Handler) Like(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	p, err := h.svc.Like(c.Request.Context(), id, middleware.Address(c))
	if err != nil {
		httpresp.AppErr(c, err)
		return
	}
	httpresp.AppOK(c, gin.H{"likeCount": p.LikeCount, "liked": p.Liked})
}

func (h *Handler) Unlike(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	p, err := h.svc.Unlike(c.Request.Context(), id, middleware.Address(c))
	if err != nil {
		httpresp.AppErr(c, err)
		return
	}
	httpresp.AppOK(c, gin.H{"likeCount": p.LikeCount, "liked": p.Liked})
}

func (h *Handler) EnableComments(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	if err := h.svc.ToggleComments(c.Request.Context(), id, middleware.Address(c), req.Enabled); err != nil {
		httpresp.AppErr(c, err)
		return
	}
	httpresp.AppOK(c, gin.H{"id": id, "enabled": req.Enabled})
}

type commentReq struct {
	Content   string `json:"content"`
	Nickname  string `json:"nickname"`
	ReplyToID int64  `json:"replyToId"`
}

func (h *Handler) CreateComment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	var req commentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	cm, err := h.svc.CreateComment(c.Request.Context(), id, middleware.Address(c), req.Nickname, req.Content, req.ReplyToID)
	if err != nil {
		httpresp.AppErr(c, err)
		return
	}
	httpresp.AppOK(c, commentJSON(cm))
}

func (h *Handler) ListComments(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	page := parsePage(c)
	thread := c.Query("thread") == "1"
	list, total, commentCount, err := h.svc.ListComments(c.Request.Context(), id, middleware.Address(c), page, thread)
	if err != nil {
		httpresp.AppErr(c, err)
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, commentJSON(&list[i]))
	}
	httpresp.AppOK(c, gin.H{"list": items, "total": total, "commentCount": commentCount, "page": page.Page, "pageSize": page.PageSize})
}

func (h *Handler) ListReplies(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || postID <= 0 {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	commentID, err := strconv.ParseInt(c.Param("commentId"), 10, 64)
	if err != nil || commentID <= 0 {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	page := parsePage(c)
	list, total, err := h.svc.ListReplies(c.Request.Context(), postID, commentID, page)
	if err != nil {
		httpresp.AppErr(c, err)
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, commentJSON(&list[i]))
	}
	httpresp.AppOK(c, gin.H{"list": items, "total": total, "page": page.Page, "pageSize": page.PageSize})
}

func (h *Handler) ListReplyInbox(c *gin.Context) {
	var read *bool
	switch c.Query("read") {
	case "0":
		v := false
		read = &v
	case "1":
		v := true
		read = &v
	case "":
	default:
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	page := parsePage(c)
	list, total, unread, err := h.svc.ListReplyInbox(c.Request.Context(), middleware.Address(c), read, page)
	if err != nil {
		httpresp.AppErr(c, err)
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, replyNoticeJSON(&list[i]))
	}
	httpresp.AppOK(c, gin.H{"list": items, "total": total, "unreadCount": unread, "page": page.Page, "pageSize": page.PageSize})
}

func (h *Handler) MarkReplyInboxRead(c *gin.Context) {
	var req struct {
		IDs []int64 `json:"ids"`
		All bool    `json:"all"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	unread, err := h.svc.MarkReplyInboxRead(c.Request.Context(), middleware.Address(c), req.All, req.IDs)
	if err != nil {
		httpresp.AppErr(c, err)
		return
	}
	httpresp.AppOK(c, gin.H{"unreadCount": unread})
}

func (h *Handler) DeleteComment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	if err := h.svc.DeleteOwnComment(c.Request.Context(), id, middleware.Address(c)); err != nil {
		httpresp.AppErr(c, err)
		return
	}
	httpresp.AppOK(c, gin.H{"id": id})
}

func (h *Handler) Follow(c *gin.Context) {
	var req struct {
		TargetAddr string `json:"targetAddr"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	if err := h.svc.Follow(c.Request.Context(), middleware.Address(c), req.TargetAddr); err != nil {
		httpresp.AppErr(c, err)
		return
	}
	httpresp.AppOK(c, gin.H{"targetAddr": strings.TrimSpace(req.TargetAddr), "followed": true})
}

func (h *Handler) Unfollow(c *gin.Context) {
	addr := strings.TrimSpace(c.Param("addr"))
	if err := h.svc.Unfollow(c.Request.Context(), middleware.Address(c), addr); err != nil {
		httpresp.AppErr(c, err)
		return
	}
	httpresp.AppOK(c, gin.H{"targetAddr": addr, "followed": false})
}

func (h *Handler) ListFollows(c *gin.Context) {
	page := parsePage(c)
	list, total, err := h.svc.ListMyFollows(c.Request.Context(), middleware.Address(c), page)
	if err != nil {
		httpresp.AppErr(c, err)
		return
	}
	httpresp.AppOK(c, gin.H{"list": followJSONList(list), "total": total, "page": page.Page, "pageSize": page.PageSize})
}

func (h *Handler) ListFollowers(c *gin.Context) {
	page := parsePage(c)
	list, total, err := h.svc.ListMyFollowers(c.Request.Context(), middleware.Address(c), page)
	if err != nil {
		httpresp.AppErr(c, err)
		return
	}
	httpresp.AppOK(c, gin.H{"list": followJSONList(list), "total": total, "page": page.Page, "pageSize": page.PageSize})
}

func (h *Handler) ListMyPosts(c *gin.Context) {
	page := parsePage(c)
	list, total, stats, err := h.svc.ListMyPosts(c.Request.Context(), middleware.Address(c), page)
	if err != nil {
		httpresp.AppErr(c, err)
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, postJSON(&list[i]))
	}
	httpresp.AppOK(c, gin.H{
		"list": items, "total": total, "page": page.Page, "pageSize": page.PageSize,
		"totalPosts": stats.TotalPosts, "totalViews": stats.TotalViews,
		"totalLikes": stats.TotalLikes, "totalComments": stats.TotalComments,
	})
}

func (h *Handler) ReportPost(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	var req struct {
		Reason string `json:"reason"`
		Detail string `json:"detail"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	r, err := h.svc.ReportPost(c.Request.Context(), middleware.Address(c), id, req.Reason, req.Detail)
	if err != nil {
		httpresp.AppErr(c, err)
		return
	}
	httpresp.AppOK(c, gin.H{"id": r.ID, "targetId": r.TargetID, "reason": r.Reason, "status": r.Status})
}

func (h *Handler) OfficialMe(c *gin.Context) {
	httpresp.AppOK(c, h.svc.OfficialMe(c.Request.Context(), middleware.Address(c)))
}

func (h *Handler) ListOfficials(c *gin.Context) {
	page := parsePage(c)
	raw := c.QueryArray("gold")
	golds := parseGoldFlags(raw)
	if len(raw) == 0 || len(golds) == 0 {
		golds = []bool{true}
	}
	list, total, err := h.svc.ListAppOfficials(c.Request.Context(), middleware.Address(c), golds, page)
	if err != nil {
		httpresp.AppErr(c, err)
		return
	}
	items := make([]gin.H, 0, len(list))
	for i := range list {
		items = append(items, appOfficialJSON(&list[i]))
	}
	httpresp.AppOK(c, gin.H{"list": items, "total": total, "page": page.Page, "pageSize": page.PageSize})
}

func (h *Handler) ApplyOfficial(c *gin.Context) {
	var req struct {
		Name   string `json:"name"`
		Avatar string `json:"avatar"`
		Intro  string `json:"intro"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	a, err := h.svc.ApplyOfficial(c.Request.Context(), middleware.Address(c), req.Name, req.Avatar, req.Intro)
	if err != nil {
		httpresp.AppErr(c, err)
		return
	}
	httpresp.AppOK(c, gin.H{"id": a.ID, "status": a.Status, "name": a.Name})
}

func (h *Handler) UpdateOfficial(c *gin.Context) {
	var req struct {
		Name   string `json:"name"`
		Avatar string `json:"avatar"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.AppErr(c, xerror.New(xerror.ParamInvalid, "参数错误"))
		return
	}
	addr := middleware.Address(c)
	if err := h.svc.UpdateMyOfficial(c.Request.Context(), addr, req.Name, req.Avatar); err != nil {
		httpresp.AppErr(c, err)
		return
	}
	httpresp.AppOK(c, gin.H{"address": addr, "name": strings.TrimSpace(req.Name), "avatar": strings.TrimSpace(req.Avatar)})
}

func parseGoldFlags(vals []string) []bool {
	seen := map[bool]struct{}{}
	out := make([]bool, 0, 2)
	for _, v := range vals {
		var b bool
		ok := true
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes":
			b = true
		case "0", "false", "no":
			b = false
		default:
			ok = false
		}
		if !ok {
			continue
		}
		if _, dup := seen[b]; dup {
			continue
		}
		seen[b] = struct{}{}
		out = append(out, b)
	}
	return out
}

func appOfficialJSON(o *entity.OfficialAccount) gin.H {
	return gin.H{
		"address": o.Address, "name": o.Name, "avatar": o.Avatar, "intro": o.Intro,
		"gold": o.Gold, "postCount": o.PostCount, "followerCount": o.FollowerCount, "followed": o.Followed,
	}
}

func parseTrimmed(vals []string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func parsePage(c *gin.Context) entity.PageQuery {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	return entity.PageQuery{Page: page, PageSize: size}.Normalize()
}

func postJSON(p *entity.Post) gin.H {
	var media []string
	_ = json.Unmarshal(p.MediaJSON, &media)
	if media == nil {
		media = []string{}
	}
	return gin.H{
		"id":              p.ID,
		"authorAddr":      p.AuthorAddr,
		"authorName":      p.AuthorName,
		"authorKind":      p.AuthorKind,
		"authorAvatar":    p.AuthorAvatar,
		"authorIntro":     p.AuthorIntro,
		"authorGold":      p.AuthorGold,
		"title":           p.Title,
		"intro":           p.Intro,
		"tag":             p.Tag,
		"content":         p.Content,
		"contentFormat":   p.ContentFormat(),
		"mediaUrls":       media,
		"commentsEnabled": p.CommentsEnabled == 1,
		"status":          p.Status,
		"commentCount":    p.CommentCount,
		"likeCount":       p.LikeCount,
		"viewCount":       p.ViewCount,
		"liked":           p.Liked,
		"pinned":          p.Pinned(),
		"pinnedAt":        pinnedAtJSON(p),
		"createdAt":       p.CreatedAt.Format(time.RFC3339),
		"editUntil":       editUntilJSON(p),
	}
}

func pinnedAtJSON(p *entity.Post) any {
	if p == nil || p.PinnedAt == nil {
		return nil
	}
	return p.PinnedAt.Format(time.RFC3339)
}

func editUntilJSON(p *entity.Post) any {
	until := usecase.PostEditUntil(p)
	if until == nil {
		return nil
	}
	return until.Format(time.RFC3339)
}

func followJSONList(list []entity.FollowProfile) []gin.H {
	items := make([]gin.H, 0, len(list))
	for i := range list {
		p := &list[i]
		items = append(items, gin.H{
			"address":    p.Addr,
			"name":       p.Name,
			"avatar":     p.Avatar,
			"targetKind": p.Kind,
			"followedAt": p.FollowedAt.Format(time.RFC3339),
		})
	}
	return items
}

func commentJSON(cm *entity.Comment) gin.H {
	out := gin.H{
		"id":          cm.ID,
		"postId":      cm.PostID,
		"authorAddr":  cm.AuthorAddr,
		"authorName":  cm.AuthorName,
		"content":     cm.Content,
		"status":      cm.Status,
		"isOfficial":  cm.AuthorAddr == entity.PlatformOfficialAddr,
		"parentId":    cm.ParentID,
		"replyToId":   cm.ReplyToID,
		"replyToAddr": cm.ReplyToAddr,
		"replyToName": cm.ReplyToName,
		"replyCount":  cm.ReplyCount,
		"deleted":     cm.Deleted,
		"createdAt":   cm.CreatedAt.Format(time.RFC3339),
	}
	if cm.Replies != nil {
		replies := make([]gin.H, 0, len(cm.Replies))
		for i := range cm.Replies {
			replies = append(replies, commentJSON(&cm.Replies[i]))
		}
		out["replies"] = replies
	}
	return out
}

func replyNoticeJSON(n *entity.ReplyNotice) gin.H {
	content := n.Content
	if n.CommentStatus != entity.StatusVisible {
		content = ""
	}
	available, postAvailable := usecase.NoticeFlags(n.PostStatus, n.CommentStatus, content)
	return gin.H{
		"id":            n.ID,
		"postId":        n.PostID,
		"postTitle":     n.PostTitle,
		"postAvailable": postAvailable,
		"rootId":        n.RootID,
		"commentId":     n.CommentID,
		"replyToId":     n.ReplyToID,
		"actorAddr":     n.ActorAddr,
		"actorName":     n.ActorName,
		"content":       content,
		"read":          n.ReadAt != nil,
		"available":     available,
		"createdAt":     n.CreatedAt.Format(time.RFC3339),
	}
}
