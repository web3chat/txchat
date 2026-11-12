package usecase

import (
	"testing"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

func TestPostEditUntilAndWindow(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	personal := &entity.Post{AuthorKind: entity.KindPersonal, CreatedAt: created}

	until := PostEditUntil(personal)
	if until == nil || !until.Equal(created.Add(ownPostEditWindow)) {
		t.Fatalf("personal until=%v", until)
	}
	if err := assertOwnPostEditWindow(personal, created); err != nil {
		t.Fatalf("just published should allow: %v", err)
	}
	if err := assertOwnPostEditWindow(personal, created.Add(ownPostEditWindow)); err != nil {
		t.Fatalf("boundary should allow: %v", err)
	}
	err := assertOwnPostEditWindow(personal, created.Add(ownPostEditWindow+time.Second))
	if err == nil {
		t.Fatal("after window should fail")
	}
	if xerror.CodeOf(err) != xerror.EditWindowClosed {
		t.Fatalf("code=%d", xerror.CodeOf(err))
	}

	oa := &entity.Post{AuthorKind: entity.KindOfficial, AuthorAddr: "oa-wallet", CreatedAt: created}
	if PostEditUntil(oa) == nil {
		t.Fatal("official account post should have edit deadline")
	}
	if err := assertOwnPostEditWindow(oa, created.Add(ownPostEditWindow+time.Second)); err == nil {
		t.Fatal("official account after window should fail")
	}

	platform := &entity.Post{
		AuthorKind: entity.KindOfficial,
		AuthorAddr: entity.PlatformOfficialAddr,
		CreatedAt:  created,
	}
	if PostEditUntil(platform) != nil {
		t.Fatal("platform official has no edit deadline")
	}
	if err := assertOwnPostEditWindow(platform, created.Add(48*time.Hour)); err != nil {
		t.Fatalf("platform official should always allow: %v", err)
	}
}
