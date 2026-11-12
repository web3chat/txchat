package usecase

import (
	"testing"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/repository"
)

func TestNextGuardCount(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		fails    int
		until    time.Time
		hasLock  bool
		wantNext int
	}{
		{name: "first failure", fails: 0, wantNext: 1},
		{name: "second failure", fails: 1, wantNext: 2},
		{name: "third failure", fails: 2, wantNext: 3},
		{name: "expired silence starts over", fails: 3, until: now.Add(-time.Minute), hasLock: true, wantNext: 1},
		{name: "active silence keeps counting", fails: 3, until: now.Add(time.Hour), hasLock: true, wantNext: 4},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := nextGuardCount(tt.fails, tt.until, tt.hasLock, now)
			if got != tt.wantNext {
				t.Fatalf("next=%d want %d", got, tt.wantNext)
			}
		})
	}
}

func TestShouldWarnLoginFails(t *testing.T) {
	t.Parallel()
	if loginAlertWindow != time.Hour || loginAlertMinIPs != 2 || loginAlertMinFails != 2 {
		t.Fatalf("window=%s ips=%d fails=%d", loginAlertWindow, loginAlertMinIPs, loginAlertMinFails)
	}
	if shouldWarnLoginFails(0) || shouldWarnLoginFails(1) {
		t.Fatal("one address should not raise the multi-ip warning")
	}
	if !shouldWarnLoginFails(2) {
		t.Fatal("two addresses should warn")
	}
}

func TestGroupSharedLogins(t *testing.T) {
	t.Parallel()
	if loginShareMinIPs != 2 {
		t.Fatalf("share min ips=%d", loginShareMinIPs)
	}
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	rows := []repository.ActiveLoginIP{
		{Username: "admin", IP: "203.0.113.10", LastSeen: now},
		{Username: "admin", IP: "203.0.113.20", LastSeen: now.Add(time.Minute)},
		{Username: "admin", IP: "203.0.113.20", LastSeen: now.Add(-time.Minute)},
		{Username: "solo", IP: "198.51.100.8", LastSeen: now},
		{Username: "reviewer", IP: "", LastSeen: now},
		{Username: "reviewer", IP: "198.51.100.9", LastSeen: now},
	}
	got := groupSharedLogins(rows, "203.0.113.10", 8)
	if len(got) != 1 {
		t.Fatalf("shares=%d", len(got))
	}
	if got[0].Username != "admin" || got[0].IPCount != 2 || len(got[0].IPs) != 2 {
		t.Fatalf("share=%+v", got[0])
	}
	if got[0].IPs[0].IP != "203.0.113.20" || got[0].IPs[0].Current {
		t.Fatalf("first ip=%+v", got[0].IPs[0])
	}
	if got[0].IPs[1].IP != "203.0.113.10" || !got[0].IPs[1].Current {
		t.Fatalf("second ip=%+v", got[0].IPs[1])
	}
	if len(groupSharedLogins(rows[:1], "", 8)) != 0 {
		t.Fatal("one address should not warn")
	}
}

func TestGroupMultiLogins(t *testing.T) {
	t.Parallel()
	if loginMultiMinSession != 2 {
		t.Fatalf("multi min sessions=%d", loginMultiMinSession)
	}
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	rows := []repository.ActiveSession{
		{ID: "a", Username: "login-test", IP: "203.0.113.30", LastSeen: now},
		{ID: "b", Username: "login-test", IP: "203.0.113.30", LastSeen: now.Add(time.Minute)},
		{ID: "b", Username: "login-test", IP: "203.0.113.30", LastSeen: now.Add(-time.Minute)},
		{ID: "c", Username: "solo", IP: "198.51.100.8", LastSeen: now},
		{ID: "d", Username: "admin", IP: "192.168.65.1", LastSeen: now},
		{ID: "e", Username: "admin", IP: "203.0.113.40", LastSeen: now.Add(time.Minute)},
		{ID: "", Username: "ghost", IP: "203.0.113.50", LastSeen: now},
	}
	got := groupMultiLogins(rows, "b", 8)
	if len(got) != 2 {
		t.Fatalf("accounts=%d", len(got))
	}
	if got[0].Username != "login-test" || got[0].SessionCount != 2 || len(got[0].Sessions) != 2 {
		t.Fatalf("same ip=%+v", got[0])
	}
	if got[0].Sessions[0].ID != "b" || !got[0].Sessions[0].Current || got[0].Sessions[0].IP != "203.0.113.30" {
		t.Fatalf("current=%+v", got[0].Sessions[0])
	}
	if got[0].Sessions[1].ID != "a" || got[0].Sessions[1].Current {
		t.Fatalf("other=%+v", got[0].Sessions[1])
	}
	if got[1].Username != "admin" || got[1].SessionCount != 2 {
		t.Fatalf("different ip=%+v", got[1])
	}
	if got[1].Sessions[0].IP != "203.0.113.40" || got[1].Sessions[1].IP != "192.168.65.1" {
		t.Fatalf("admin sessions=%+v", got[1].Sessions)
	}
	if len(groupMultiLogins(rows[3:4], "", 8)) != 0 {
		t.Fatal("one session should not warn")
	}
}

func TestLoginIPSilencePolicy(t *testing.T) {
	t.Parallel()
	if loginIPLockAfter != 3 {
		t.Fatalf("ip lock after=%d", loginIPLockAfter)
	}
	if loginIPLockFor != time.Hour {
		t.Fatalf("ip lock for=%s", loginIPLockFor)
	}
}
