package dao

import "testing"

func TestBuildAuditWhereParallelFilters(t *testing.T) {
	where, args := buildAuditWhere(
		[]string{"login_ok", " delete_whitelist ", "login_ok"},
		[]string{"group", "whitelist"},
		" 1001 ",
		" addr ",
	)
	want := " WHERE action IN (?,?) AND target_type IN (?,?) AND target_id = ? AND operator = ?"
	if where != want {
		t.Fatalf("where=%q", where)
	}
	if len(args) != 6 {
		t.Fatalf("args=%v", args)
	}
	if args[0] != "login_ok" || args[1] != "delete_whitelist" || args[2] != "group" || args[3] != "whitelist" || args[4] != "1001" || args[5] != "addr" {
		t.Fatalf("args=%v", args)
	}
}

func TestBuildAuditWhereEmpty(t *testing.T) {
	where, args := buildAuditWhere(nil, []string{" ", ""}, "", "")
	if where != "" || len(args) != 0 {
		t.Fatalf("where=%q args=%v", where, args)
	}
}

func TestClipAuditRunes(t *testing.T) {
	got := clipAudit("  删除白名单记录  ", 4)
	if got != "删除白名" {
		t.Fatalf("got=%q", got)
	}
}
