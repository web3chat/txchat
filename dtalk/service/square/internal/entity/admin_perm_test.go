package entity

import (
	"reflect"
	"testing"
)

func TestDefaultAndFormatPerms(t *testing.T) {
	t.Parallel()
	super := DefaultPerms(RoleSuper, false)
	if FormatPerms(super) != "official_ban,gold,settings" {
		t.Fatalf("super=%v", super)
	}
	root := DefaultPerms(RoleSuper, true)
	if FormatPerms(root) != "official_ban,gold,settings,staff" {
		t.Fatalf("root=%v", root)
	}
	if FormatPerms(DefaultPerms(RoleReviewer, false)) != "" {
		t.Fatal("reviewer should start with no extra perms")
	}
}

func TestParsePermsOrderAndDedupe(t *testing.T) {
	t.Parallel()
	got := ParsePerms(" gold, settings,gold ,staff ")
	want := []string{PermGold, PermSettings, PermStaff}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%v", got)
	}
	if ParsePerms("") != nil && len(ParsePerms("")) != 0 {
		t.Fatal("empty")
	}
}

func TestNormalizeAssignable(t *testing.T) {
	t.Parallel()
	got, ok := NormalizeAssignable([]string{"settings", "gold", "gold"})
	if !ok || !reflect.DeepEqual(got, []string{PermGold, PermSettings}) {
		t.Fatalf("ok=%v got=%v", ok, got)
	}
	if _, ok := NormalizeAssignable([]string{"staff"}); ok {
		t.Fatal("staff must not be assignable")
	}
	if _, ok := NormalizeAssignable([]string{"nope"}); ok {
		t.Fatal("unknown must fail")
	}
}

func TestEffectivePerms(t *testing.T) {
	t.Parallel()
	ready := &Admin{Role: RoleSuper, PermReady: true, Permissions: []string{PermGold}}
	if FormatPerms(ready.EffectivePerms()) != PermGold {
		t.Fatalf("ready=%v", ready.EffectivePerms())
	}
	legacy := &Admin{Role: RoleSuper}
	if FormatPerms(legacy.EffectivePerms()) != "official_ban,gold,settings" {
		t.Fatalf("legacy=%v", legacy.EffectivePerms())
	}
}
