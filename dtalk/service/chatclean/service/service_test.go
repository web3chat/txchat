package service

import (
	"testing"
	"time"
)

func TestDateOptionToRange_Custom(t *testing.T) {
	start := int64(1700000000000)
	end := int64(1700100000000)

	gotStart, gotEnd := dateOptionToRange(DateOptionCustom, start, end)
	if gotStart != start || gotEnd != end {
		t.Fatalf("custom range mismatch: got (%d,%d), want (%d,%d)", gotStart, gotEnd, start, end)
	}
}

func TestDateOptionToRange_AllTime(t *testing.T) {
	before := time.Now().UnixMilli()
	gotStart, gotEnd := dateOptionToRange(DateOptionAllTime, 0, 0)
	after := time.Now().UnixMilli()

	if gotStart != 0 {
		t.Fatalf("all time start mismatch: got %d, want 0", gotStart)
	}
	if gotEnd < before || gotEnd > after {
		t.Fatalf("all time end out of expected range: got %d, want [%d,%d]", gotEnd, before, after)
	}
}

func TestDateOptionToRange_Last7Days(t *testing.T) {
	before := time.Now().UnixMilli()
	gotStart, gotEnd := dateOptionToRange(DateOptionLast7Days, 0, 0)
	after := time.Now().UnixMilli()

	expectedDurationMs := int64(7 * 24 * time.Hour / time.Millisecond)
	actualDuration := gotEnd - gotStart
	delta := actualDuration - expectedDurationMs
	if delta < 0 {
		delta = -delta
	}

	if gotEnd < before || gotEnd > after {
		t.Fatalf("last7days end out of expected range: got %d, want [%d,%d]", gotEnd, before, after)
	}
	// 允许少量执行时间偏差
	if delta > 1000 {
		t.Fatalf("last7days duration mismatch: got %dms, want about %dms", actualDuration, expectedDurationMs)
	}
}

func TestNormalizeUidPair(t *testing.T) {
	tests := []struct {
		name  string
		uid1  string
		uid2  string
		wantA string
		wantB string
	}{
		{name: "already ordered", uid1: "a", uid2: "b", wantA: "a", wantB: "b"},
		{name: "need swap", uid1: "b", uid2: "a", wantA: "a", wantB: "b"},
		{name: "same uid", uid1: "a", uid2: "a", wantA: "a", wantB: "a"},
		{name: "numeric strings", uid1: "20", uid2: "3", wantA: "20", wantB: "3"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotA, gotB := normalizeUidPair(tc.uid1, tc.uid2)
			if gotA != tc.wantA || gotB != tc.wantB {
				t.Fatalf("normalizeUidPair(%q,%q) = (%q,%q), want (%q,%q)",
					tc.uid1, tc.uid2, gotA, gotB, tc.wantA, tc.wantB)
			}
		})
	}
}
