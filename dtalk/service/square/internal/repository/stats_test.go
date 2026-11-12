package repository

import (
	"testing"
	"time"
)

func TestMergeDailySeriesFillsGaps(t *testing.T) {
	t.Parallel()
	loc := time.FixedZone("CST", 8*3600)
	from := time.Date(2026, 9, 13, 10, 0, 0, 0, loc)
	last := time.Date(2026, 9, 15, 23, 0, 0, 0, loc)
	got := mergeDailySeries(from, last,
		map[string]int64{"2026-09-13": 2, "2026-09-15": 5},
		map[string]int64{"2026-09-14": 3},
		map[string]int64{"2026-09-15": 9},
		map[string]int64{},
	)
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	wantDates := []string{"2026-09-13", "2026-09-14", "2026-09-15"}
	wantPosts := []int64{2, 0, 5}
	wantComments := []int64{0, 3, 0}
	wantViews := []int64{0, 0, 9}
	for i, day := range got {
		if day.Date != wantDates[i] {
			t.Fatalf("date[%d]=%s want %s", i, day.Date, wantDates[i])
		}
		if day.Posts != wantPosts[i] || day.Comments != wantComments[i] || day.Views != wantViews[i] || day.Likes != 0 {
			t.Fatalf("counts[%d]=%+v", i, day)
		}
	}
}

func TestMergeDailySeriesEmptyRange(t *testing.T) {
	t.Parallel()
	from := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	last := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	got := mergeDailySeries(from, last, nil, nil, nil, nil)
	if len(got) != 0 {
		t.Fatalf("len=%d want 0", len(got))
	}
}
