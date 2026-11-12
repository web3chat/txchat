package usecase

import "testing"

func TestNormalizeFollowTarget(t *testing.T) {
	t.Parallel()
	if _, err := normalizeFollowTarget("a", "a"); err == nil {
		t.Fatal("self follow should fail")
	}
	if _, err := normalizeFollowTarget("a", ""); err == nil {
		t.Fatal("empty target should fail")
	}
	got, err := normalizeFollowTarget("alice", " bob ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "bob" {
		t.Fatalf("got %s", got)
	}
}

func TestValidReportReason(t *testing.T) {
	t.Parallel()
	if !validReportReason("spam") || validReportReason("xxx") {
		t.Fatal("reason table mismatch")
	}
}
