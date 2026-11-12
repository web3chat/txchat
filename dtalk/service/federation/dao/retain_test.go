package dao

import (
	"testing"
	"time"
)

func TestRetainExpired(t *testing.T) {
	now := time.UnixMilli(1762401840000)
	cutoff := now.Add(-retentionKeep).UnixMilli()
	old := uint64(cutoff - 1)
	recent := uint64(now.Add(-time.Hour).UnixMilli())
	cases := []struct {
		name       string
		mid        int64
		createTime uint64
		want       bool
	}{
		{name: "早于 7 天", mid: retainMid(int64(old)), createTime: old, want: true},
		{name: "7 天内", mid: retainMid(int64(recent)), createTime: recent, want: false},
		{name: "等于截止", mid: retainMid(cutoff), createTime: uint64(cutoff), want: false},
		{name: "时间为 0", mid: retainMid(int64(old)), createTime: 0, want: false},
		{name: "时间旧 mid 新", mid: retainMid(int64(recent)), createTime: old, want: false},
		{name: "mid 旧时间新", mid: retainMid(int64(old)), createTime: recent, want: false},
		{name: "mid 为 0", mid: 0, createTime: old, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := retainExpired(tc.mid, tc.createTime, now); got != tc.want {
				t.Fatalf("retainExpired = %v, want %v", got, tc.want)
			}
		})
	}
}

func retainMid(ms int64) int64 {
	delta := ms - retentionEpoch
	if delta <= 0 {
		return 1
	}
	return delta << retentionShift
}
