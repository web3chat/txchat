package usecase

import (
	"strings"
	"testing"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

func TestResolveReplyMount(t *testing.T) {
	t.Parallel()
	root := &entity.Comment{ID: 101, PostID: 12, ParentID: 0, Status: entity.StatusVisible, AuthorAddr: "ming", AuthorName: "小明"}
	reply := &entity.Comment{ID: 102, PostID: 12, ParentID: 101, Status: entity.StatusVisible, AuthorAddr: "hong", AuthorName: "小红"}
	hidden := &entity.Comment{ID: 103, PostID: 12, ParentID: 0, Status: entity.StatusHidden, AuthorAddr: "li", AuthorName: "小李"}
	otherPost := &entity.Comment{ID: 104, PostID: 99, ParentID: 0, Status: entity.StatusVisible, AuthorAddr: "zhao", AuthorName: "小赵"}

	cases := []struct {
		name      string
		replyToID int64
		postID    int64
		target    *entity.Comment
		want      ReplyMount
		wantCode  int
	}{
		{name: "top", replyToID: 0, postID: 12, want: ReplyMount{}},
		{name: "reply root", replyToID: 101, postID: 12, target: root, want: ReplyMount{ParentID: 101, ReplyToID: 101, ReplyToAddr: "ming", ReplyToName: "小明"}},
		{name: "reply reply lifts parent", replyToID: 102, postID: 12, target: reply, want: ReplyMount{ParentID: 101, ReplyToID: 102, ReplyToAddr: "hong", ReplyToName: "小红"}},
		{name: "hidden", replyToID: 103, postID: 12, target: hidden, wantCode: xerror.ReplyUnavailable},
		{name: "missing", replyToID: 1, postID: 12, target: nil, wantCode: xerror.ReplyUnavailable},
		{name: "other post", replyToID: 104, postID: 12, target: otherPost, wantCode: xerror.ReplyUnavailable},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveReplyMount(tc.replyToID, tc.postID, tc.target)
			if tc.wantCode != 0 {
				if xerror.CodeOf(err) != tc.wantCode {
					t.Fatalf("code=%d want=%d err=%v", xerror.CodeOf(err), tc.wantCode, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got=%+v want=%+v", got, tc.want)
			}
		})
	}
}

func TestNoticeRecipient(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		author     string
		postAuthor string
		replyTo    string
		parentID   int64
		want       string
	}{
		{name: "top notifies post author", author: "hong", postAuthor: "ming", want: "ming"},
		{name: "self comment", author: "ming", postAuthor: "ming", want: ""},
		{name: "official post", author: "hong", postAuthor: entity.PlatformOfficialAddr, want: ""},
		{name: "reply notifies reply target", author: "gang", postAuthor: "ming", replyTo: "hong", parentID: 101, want: "hong"},
		{name: "reply self", author: "hong", postAuthor: "ming", replyTo: "hong", parentID: 101, want: ""},
		{name: "reply official target", author: "hong", postAuthor: "ming", replyTo: entity.PlatformOfficialAddr, parentID: 101, want: ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := NoticeRecipient(tc.author, tc.postAuthor, tc.replyTo, tc.parentID); got != tc.want {
				t.Fatalf("got=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestPostTitleSnapshot(t *testing.T) {
	t.Parallel()
	if got := PostTitleSnapshot("周末去哪", "简介", "正文"); got != "周末去哪" {
		t.Fatalf("title got=%q", got)
	}
	if got := PostTitleSnapshot("  ", "简介", "正文"); got != "简介" {
		t.Fatalf("intro got=%q", got)
	}
	if got := PostTitleSnapshot("", "", "正文前缀"); got != "正文前缀" {
		t.Fatalf("content got=%q", got)
	}
	long := strings.Repeat("字", maxTitleRunes+5)
	if got := PostTitleSnapshot("", "", long); len([]rune(got)) != maxTitleRunes {
		t.Fatalf("runes=%d", len([]rune(got)))
	}
}

func TestVisibilityDelta(t *testing.T) {
	t.Parallel()
	if got := VisibilityDelta(0, false, true); got != (CountDelta{Comment: 1}) {
		t.Fatalf("top show %+v", got)
	}
	if got := VisibilityDelta(101, false, true); got != (CountDelta{Comment: 1, Reply: 1}) {
		t.Fatalf("reply show %+v", got)
	}
	if got := VisibilityDelta(101, true, false); got != (CountDelta{Comment: -1, Reply: -1}) {
		t.Fatalf("reply hide %+v", got)
	}
	if got := VisibilityDelta(0, true, true); got != (CountDelta{}) {
		t.Fatalf("noop %+v", got)
	}
}

func TestNoticeFlags(t *testing.T) {
	t.Parallel()
	available, postOK := NoticeFlags(entity.StatusVisible, entity.StatusVisible, "同意")
	if !available || !postOK {
		t.Fatal("visible notice should be available")
	}
	available, postOK = NoticeFlags(entity.StatusVisible, entity.StatusHidden, "")
	if available || !postOK {
		t.Fatal("hidden comment is not unread-able")
	}
	available, postOK = NoticeFlags(entity.StatusDeleted, entity.StatusVisible, "这帖不错")
	if available || postOK {
		t.Fatal("deleted post is not available")
	}
}

func TestCommentListCounts(t *testing.T) {
	t.Parallel()
	total, commentCount := commentListCounts(false, 4, 9)
	if total != 4 || commentCount != 4 {
		t.Fatalf("flat total=%d commentCount=%d", total, commentCount)
	}
	total, commentCount = commentListCounts(true, 2, 9)
	if total != 2 || commentCount != 9 {
		t.Fatalf("thread total=%d commentCount=%d", total, commentCount)
	}
}
