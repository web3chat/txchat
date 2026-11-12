package repository

import "testing"

func TestBanOrderBy(t *testing.T) {
	t.Parallel()
	if got := banOrderBy(""); got != " ORDER BY created_at DESC, address DESC" {
		t.Fatalf("got %q", got)
	}
	if got := banOrderBy("asc"); got != " ORDER BY created_at ASC, address ASC" {
		t.Fatalf("got %q", got)
	}
}
