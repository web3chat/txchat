package repository

import (
	"context"
	"strings"
	"testing"
)

func TestOfficialAvatarMapEmpty(t *testing.T) {
	t.Parallel()
	s := &Store{}
	m, err := s.OfficialAvatarMap(context.Background(), []string{"", "  "})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 0 {
		t.Fatalf("got %v", m)
	}
}

func TestOfficialProfileMapEmpty(t *testing.T) {
	t.Parallel()
	s := &Store{}
	m, err := s.OfficialProfileMap(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 0 {
		t.Fatalf("got %v", m)
	}
}

func TestOfficialWhereGold(t *testing.T) {
	t.Parallel()
	_, oWhere, args := officialWhere(OfficialListFilter{Statuses: []int16{1}, Golds: []bool{true}})
	if !strings.Contains(oWhere, "o.gold=") {
		t.Fatalf("where=%s", oWhere)
	}
	if len(args) != 2 || args[1] != true {
		t.Fatalf("args=%v", args)
	}
	_, both, bothArgs := officialWhere(OfficialListFilter{Golds: []bool{true, false}})
	if strings.Contains(both, "gold") {
		t.Fatalf("both golds should not filter: %s", both)
	}
	if len(bothArgs) != 0 {
		t.Fatalf("both args=%v", bothArgs)
	}
}

func TestOfficialOrderBy(t *testing.T) {
	t.Parallel()
	if !strings.Contains(officialOrderBy("followers"), "fc.n") {
		t.Fatal("followers")
	}
	if !strings.Contains(officialOrderBy("posts"), "pc.n") {
		t.Fatal("posts")
	}
	if !strings.Contains(officialOrderBy("reports"), "rc.n") {
		t.Fatal("reports")
	}
	if strings.Contains(officialOrderBy("nope"), "rc.n") {
		t.Fatal("unknown order")
	}
	if !strings.Contains(officialOrderBy(""), "created_at") {
		t.Fatal("default")
	}
}
