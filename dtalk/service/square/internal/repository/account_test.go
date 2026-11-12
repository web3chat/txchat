package repository

import (
	"strings"
	"testing"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

func TestSQLAuthorRankOrder(t *testing.T) {
	t.Parallel()
	if got := sqlAuthorRankOrder("likes"); !strings.Contains(got, "g.like_count DESC") {
		t.Fatalf("likes=%s", got)
	}
	if got := sqlAuthorRankOrder("views"); !strings.Contains(got, "g.view_count DESC") {
		t.Fatalf("views=%s", got)
	}
	if got := sqlAuthorRankOrder("hack"); !strings.Contains(got, "g.like_count DESC") {
		t.Fatalf("default=%s", got)
	}
	if strings.Contains(sqlAuthorRankOrder("likes;drop"), ";") {
		t.Fatal("order injection")
	}
}

func TestAuthorRankInnerWhere(t *testing.T) {
	t.Parallel()
	from := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	where, args := authorRankInnerWhere(AuthorRankFilter{
		From:        &from,
		AuthorAddrs: []string{"a1", " a2 ", ""},
	})
	if !strings.Contains(where, "p.created_at >= $3") {
		t.Fatalf("from: %s", where)
	}
	if !strings.Contains(where, "p.author_addr IN ($4,$5)") {
		t.Fatalf("addrs: %s", where)
	}
	if len(args) != 5 || args[0] != entity.StatusDeleted || args[1] != entity.StatusDraft {
		t.Fatalf("args=%v", args)
	}
}

func TestAuthorRankOuterWhere(t *testing.T) {
	t.Parallel()
	where, args := authorRankOuterWhere([]string{"广场", "官方"}, []any{"keep"})
	if !strings.Contains(where, "oa.name ILIKE") || !strings.Contains(where, "nm.author_name ILIKE") {
		t.Fatalf("names: %s", where)
	}
	if len(args) != 5 || args[0] != "keep" {
		t.Fatalf("args=%v", args)
	}
	empty, keep := authorRankOuterWhere(nil, []any{1})
	if empty != "" || len(keep) != 1 {
		t.Fatalf("empty=%q keep=%v", empty, keep)
	}
}
