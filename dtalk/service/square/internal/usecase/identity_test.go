package usecase

import (
	"testing"
	"time"
)

func TestPostRateWindows(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		official bool
		gold     bool
		wantDay  int64
		unlimited bool
	}{
		{name: "个人用户每天20条", official: false, gold: false, wantDay: 20},
		{name: "公众号每天10条", official: true, gold: false, wantDay: officialPostDayLimit},
		{name: "金标公众号不限", official: true, gold: true, unlimited: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := postRateWindows(tc.official, tc.gold)
			if tc.unlimited {
				if len(got) != 0 {
					t.Fatalf("gold official should have no windows, got %+v", got)
				}
				return
			}
			if len(got) != 2 {
				t.Fatalf("windows=%d, want 2", len(got))
			}
			if got[0].dur != 10*time.Minute || got[0].limit != 5 {
				t.Fatalf("short window = %+v, want 10m/5", got[0])
			}
			if got[1].dur != 24*time.Hour || got[1].limit != tc.wantDay {
				t.Fatalf("day window = %+v, want 24h/%d", got[1], tc.wantDay)
			}
		})
	}
}
