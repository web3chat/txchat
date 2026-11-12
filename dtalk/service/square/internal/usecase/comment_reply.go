package usecase

import (
	"strings"
	"unicode/utf8"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

const replyPreviewLimit = 2

// ReplyMount 是写入 square_comment 的两级挂载。ParentID 只指向一级评论。
type ReplyMount struct {
	ParentID    int64
	ReplyToID   int64
	ReplyToAddr string
	ReplyToName string
}

// ResolveReplyMount 把 replyToId 收成两级。replyToID<=0 表示一级评论。
func ResolveReplyMount(replyToID, postID int64, target *entity.Comment) (ReplyMount, error) {
	if replyToID <= 0 {
		return ReplyMount{}, nil
	}
	if target == nil || target.PostID != postID || target.Status != entity.StatusVisible {
		return ReplyMount{}, xerror.New(xerror.ReplyUnavailable, "不能回复该评论")
	}
	parent := target.ID
	if target.ParentID > 0 {
		parent = target.ParentID
	}
	return ReplyMount{
		ParentID:    parent,
		ReplyToID:   target.ID,
		ReplyToAddr: target.AuthorAddr,
		ReplyToName: target.AuthorName,
	}, nil
}

// NoticeRecipient 一级评论通知帖子作者，回复通知被回复的人。自己和平台官方不写收件箱。
func NoticeRecipient(authorAddr, postAuthor, replyToAddr string, parentID int64) string {
	var to string
	if parentID == 0 {
		to = strings.TrimSpace(postAuthor)
	} else {
		to = strings.TrimSpace(replyToAddr)
	}
	authorAddr = strings.TrimSpace(authorAddr)
	if to == "" || to == authorAddr || to == entity.PlatformOfficialAddr {
		return ""
	}
	return to
}

// PostTitleSnapshot 标题为空时用简介，再空则用正文，最多 64 个字。
func PostTitleSnapshot(title, intro, content string) string {
	if s := clipRunes(strings.TrimSpace(title), maxTitleRunes); s != "" {
		return s
	}
	if s := clipRunes(strings.TrimSpace(intro), maxTitleRunes); s != "" {
		return s
	}
	return clipRunes(strings.TrimSpace(content), maxTitleRunes)
}

func clipRunes(s string, n int) string {
	if n <= 0 || s == "" || utf8.RuneCountInString(s) <= n {
		return s
	}
	i := 0
	for k := range s {
		if i == n {
			return s[:k]
		}
		i++
	}
	return s
}

// CountDelta 可见性变化时帖子评论数和楼回复数的差额。
type CountDelta struct {
	Comment int64
	Reply   int64
}

func VisibilityDelta(parentID int64, wasVisible, nowVisible bool) CountDelta {
	if wasVisible == nowVisible {
		return CountDelta{}
	}
	d := int64(1)
	if wasVisible {
		d = -1
	}
	out := CountDelta{Comment: d}
	if parentID > 0 {
		out.Reply = d
	}
	return out
}

// commentListCounts 默认列表的 total 与 commentCount 相同；thread=1 时 total 是楼数，commentCount 含回复。
func commentListCounts(thread bool, listTotal, postCommentCount int64) (total, commentCount int64) {
	if thread {
		return listTotal, postCommentCount
	}
	return listTotal, listTotal
}

// NoticeFlags 收件箱是否还能点进帖子，以及帖子本身是否仍可见。
func NoticeFlags(postStatus, commentStatus int16, content string) (available, postAvailable bool) {
	postAvailable = postStatus == entity.StatusVisible
	available = postAvailable && commentStatus == entity.StatusVisible && strings.TrimSpace(content) != ""
	return available, postAvailable
}
