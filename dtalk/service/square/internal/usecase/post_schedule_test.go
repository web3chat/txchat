package usecase

import (
	"testing"
	"time"
)

func TestResolveOfficialTiming(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 6, 8, 0, 0, 0, time.UTC)
	inTwoDays := now.Add(48 * time.Hour)
	tooSoon := now.Add(30 * time.Second)
	tooFar := now.Add(15 * 24 * time.Hour)
	tests := []struct {
		name       string
		live       bool
		draft      bool
		publishAt  *time.Time
		expireSec  int64
		wantStatus int16
		scheduled  bool
		wantExpire bool
		wantErr    bool
	}{
		{name: "publish now", expireSec: 12 * 3600, wantStatus: 1, wantExpire: true},
		{name: "draft keeps duration", draft: true, expireSec: 24 * 3600, wantStatus: 6},
		{name: "schedule", publishAt: &inTwoDays, expireSec: 7 * 24 * 3600, wantStatus: 6, scheduled: true},
		{name: "too soon", publishAt: &tooSoon, wantErr: true},
		{name: "over two weeks", publishAt: &tooFar, wantErr: true},
		{name: "live cannot schedule", live: true, publishAt: &inTwoDays, wantErr: true},
		{name: "live expire from now", live: true, expireSec: 86400, wantStatus: 1, wantExpire: true},
		{name: "live clear expire", live: true, wantStatus: 1},
		{name: "expire too short", expireSec: 60, wantErr: true},
		{name: "expire over 30 days", expireSec: 31 * 24 * 3600, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveOfficialTiming(now, tt.live, tt.draft, tt.publishAt, tt.expireSec)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got.Status != tt.wantStatus || got.Scheduled != tt.scheduled {
				t.Fatalf("status=%d scheduled=%v", got.Status, got.Scheduled)
			}
			if tt.scheduled && (got.PublishAt == nil || got.ExpireAt != nil) {
				t.Fatal("scheduled post should keep publishAt and delay expireAt")
			}
			if (got.ExpireAt != nil) != tt.wantExpire {
				t.Fatalf("expireAt set=%v", got.ExpireAt != nil)
			}
			if got.ExpireAfterSec != tt.expireSec && tt.expireSec >= 0 {
				t.Fatalf("expireAfter=%d", got.ExpireAfterSec)
			}
		})
	}
}
