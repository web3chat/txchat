package service

import (
	"testing"
	"time"
)

func TestLoginLogRetentionCutoff(t *testing.T) {
	t.Parallel()
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 11, 10, 13, 21, 0, 0, loc)
	got := loginLogRetentionCutoff(now)
	want := time.Date(2026, 9, 10, 13, 21, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("cutoff = %s, want %s", got, want)
	}
}
