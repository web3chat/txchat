package repository

import (
	"strings"
	"testing"
)

func TestSQLOrderBy(t *testing.T) {
	t.Parallel()
	if got := sqlOrderBy("hot", "DESC"); got != " ORDER BY (p.like_count * 2 + p.comment_count * 3) DESC, p.id DESC" {
		t.Fatalf("hot=%s", got)
	}
	if got := sqlOrderBy("likes", "DESC"); got != " ORDER BY p.like_count DESC, p.id DESC" {
		t.Fatalf("likes=%s", got)
	}
	if got := sqlOrderBy("comments", "DESC"); got != " ORDER BY p.comment_count DESC, p.id DESC" {
		t.Fatalf("comments=%s", got)
	}
	if got := sqlOrderBy("followers", "DESC"); !strings.Contains(got, "author_followers.n") {
		t.Fatalf("followers=%s", got)
	}
	if got := sqlOrderBy("posts", "DESC"); !strings.Contains(got, "author_posts.n") {
		t.Fatalf("posts=%s", got)
	}
	if got := sqlOrderBy("asc", "DESC"); got != " ORDER BY p.created_at ASC, p.id ASC" {
		t.Fatalf("asc=%s", got)
	}
	if got := sqlOrderByCreatedAt("desc", "ASC"); got != " ORDER BY created_at DESC, id DESC" {
		t.Fatalf("comment order=%s", got)
	}
}

func TestPostWhereAuthorNameExact(t *testing.T) {
	t.Parallel()
	_, where, args := postFromWhere(PostListFilter{AuthorNameExact: []string{"甲", "乙"}})
	if !strings.Contains(where, "p.author_name IN ($1,$2)") {
		t.Fatalf("where=%s", where)
	}
	if len(args) != 2 || args[0] != "甲" || args[1] != "乙" {
		t.Fatalf("args=%v", args)
	}
}

func TestPostFromWhereFollowing(t *testing.T) {
	t.Parallel()
	from, where, args := postFromWhere(PostListFilter{OnlyVisible: true, FollowerAddr: "abc"})
	if from == "FROM square_post p" {
		t.Fatal("expected follow join")
	}
	if len(args) == 0 || args[0] != "abc" {
		t.Fatalf("args=%v", args)
	}
	if where == "" {
		t.Fatal("empty where")
	}
}

func TestPostWhereGoldOnly(t *testing.T) {
	t.Parallel()
	_, where, _ := postFromWhere(PostListFilter{GoldOnly: true})
	if !strings.Contains(where, "square_official_account WHERE gold") {
		t.Fatalf("gold only: %s", where)
	}
	from, _, _ := postFromWhere(PostListFilter{Order: "followers"})
	if !strings.Contains(from, "author_followers") || !strings.Contains(from, "author_posts") {
		t.Fatalf("author stat join: %s", from)
	}
}

func TestPostWhereTags(t *testing.T) {
	t.Parallel()
	_, where, args := postFromWhere(PostListFilter{Tags: []string{"公告", " 新闻 ", ""}})
	if !strings.Contains(where, "p.tag IN (") {
		t.Fatalf("tags where: %s", where)
	}
	if len(args) != 2 || args[0] != "公告" || args[1] != "消息" {
		t.Fatalf("tags args=%v", args)
	}
	_, empty, emptyArgs := postFromWhere(PostListFilter{Tags: []string{"", "  "}})
	if strings.Contains(empty, "p.tag") {
		t.Fatalf("blank tags should not filter: %s args=%v", empty, emptyArgs)
	}
}

func TestPostWherePinnedFlags(t *testing.T) {
	t.Parallel()
	_, where, _ := postFromWhere(PostListFilter{OnlyVisible: true, ExcludePinned: true})
	if !strings.Contains(where, "p.pinned_at IS NULL") {
		t.Fatalf("exclude pinned: %s", where)
	}
	_, pinWhere, _ := postFromWhere(PostListFilter{OnlyVisible: true, PinnedOnly: true})
	if !strings.Contains(pinWhere, "p.pinned_at IS NOT NULL") {
		t.Fatalf("pinned only: %s", pinWhere)
	}
}
