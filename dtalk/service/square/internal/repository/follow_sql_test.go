package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

func TestListFollowersEncodesKind(t *testing.T) {
	dsn := os.Getenv("SQUARE_TEST_DSN")
	if dsn == "" {
		t.Skip("SQUARE_TEST_DSN unset")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	const follower, target = "kind-follower", "kind-target"
	cleanup := func() {
		if _, err := db.ExecContext(ctx, `DELETE FROM square_comment WHERE author_addr=$1`, follower); err != nil {
			t.Errorf("cleanup comment: %v", err)
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM square_post WHERE author_addr=$1`, follower); err != nil {
			t.Errorf("cleanup post: %v", err)
		}
		if _, err := db.ExecContext(ctx, `DELETE FROM square_follow WHERE follower_addr=$1 OR target_addr=$2`, follower, target); err != nil {
			t.Errorf("cleanup follow: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	if _, err = db.ExecContext(ctx, `INSERT INTO square_follow (follower_addr, target_addr) VALUES ($1,$2)`, follower, target); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO square_post (author_addr, author_name, content, status) VALUES ($1,'昵称甲','hello',1)`, follower); err != nil {
		t.Fatal(err)
	}

	store := New(db)
	page := entity.PageQuery{Page: 1, PageSize: 50}
	list, total, err := store.ListFollowers(ctx, target, page)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("total=%d len=%d", total, len(list))
	}
	got := list[0]
	if got.Addr != follower || got.Kind != entity.KindPersonal || got.Name != "昵称甲" || got.FollowedAt.IsZero() {
		t.Fatalf("profile=%+v", got)
	}
	if got.FollowedAt.After(time.Now().Add(time.Minute)) {
		t.Fatalf("followedAt=%s", got.FollowedAt)
	}

	follows, n, err := store.ListFollows(ctx, follower, page)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || len(follows) != 1 || follows[0].Addr != target || follows[0].Kind != entity.KindPersonal {
		t.Fatalf("follows n=%d list=%+v", n, follows)
	}
}
