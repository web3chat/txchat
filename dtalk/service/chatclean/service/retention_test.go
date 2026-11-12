package service

import (
	"context"
	"testing"
	"time"
)

func TestCutoffBoundary(t *testing.T) {
	now := time.UnixMilli(1762401840000) // 2026-11-06 03:24:00 UTC，本地时区另测日期
	keep := 168 * time.Hour
	cutoffMs, cutoffMid := Cutoff(now, keep)
	wantMs := now.Add(-keep).UnixMilli()
	if cutoffMs != wantMs {
		t.Fatalf("cutoff ms = %d, want %d", cutoffMs, wantMs)
	}
	if cutoffMid != (wantMs-snowflakeEpoch)<<snowflakeTimeShift {
		t.Fatalf("cutoff mid = %d", cutoffMid)
	}

	recentMs := now.Add(-167 * time.Hour).UnixMilli()
	oldMs := now.Add(-169 * time.Hour).UnixMilli()
	cases := []struct {
		name       string
		mid        int64
		createTime int64
		want       bool
	}{
		{name: "刚好 7 天内", mid: snowflakeAt(recentMs), createTime: recentMs, want: false},
		{name: "早于 7 天", mid: snowflakeAt(oldMs), createTime: oldMs, want: true},
		{name: "等于截止时刻", mid: snowflakeAt(cutoffMs), createTime: cutoffMs, want: false},
		{name: "早 1 毫秒", mid: snowflakeAt(cutoffMs - 1), createTime: cutoffMs - 1, want: true},
		{name: "时间旧但 mid 新", mid: snowflakeAt(recentMs), createTime: oldMs, want: false},
		{name: "mid 旧但时间新", mid: snowflakeAt(oldMs), createTime: recentMs, want: false},
		{name: "时间为 0", mid: snowflakeAt(oldMs), createTime: 0, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := expiredMessage(tc.mid, tc.createTime, cutoffMid, cutoffMs)
			if got != tc.want {
				t.Fatalf("expired = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCutoffDayKeepsCutoffDate(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 11, 6, 11, 24, 0, 0, loc)
	day := CutoffDay(now, 168*time.Hour)
	if day != "20261030" {
		t.Fatalf("cutoff day = %s, want 20261030", day)
	}
	if DateFolderExpired("20261030", day) {
		t.Fatal("cutoff day should stay")
	}
	if !DateFolderExpired("20261029", day) {
		t.Fatal("previous day should expire")
	}
	if DateFolderExpired("20261101", day) {
		t.Fatal("later day should stay")
	}
	if DateFolderExpired("avatar", day) || DateFolderExpired("2026103", day) {
		t.Fatal("non date folder should stay")
	}
}

func TestPurgeMaxBatchesThenContinue(t *testing.T) {
	old := retentionPause
	retentionPause = 0
	t.Cleanup(func() { retentionPause = old })

	store := &memExpired{content: 2, orphan: 1}
	ctx := context.Background()

	first, err := purgeChannel(ctx, store, "dtalk_msg_content", "dtalk_msg_relation", 1, 1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if first.Deleted != 1 || !first.More {
		t.Fatalf("first round = %+v, want deleted 1 and more", first)
	}
	if store.orphanCalls != 0 {
		t.Fatalf("orphan calls = %d, want 0 while content remains", store.orphanCalls)
	}
	if store.content != 1 {
		t.Fatalf("content left = %d, want 1", store.content)
	}

	var last channelResult
	for i := 0; i < 10; i++ {
		last, err = purgeChannel(ctx, store, "dtalk_msg_content", "dtalk_msg_relation", 1, 1, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if !last.More && store.content == 0 && store.orphan == 0 {
			return
		}
	}
	t.Fatalf("not drained: content=%d orphan=%d last=%+v", store.content, store.orphan, last)
}

func TestPurgeOrphanWhenContentAlreadyGone(t *testing.T) {
	old := retentionPause
	retentionPause = 0
	t.Cleanup(func() { retentionPause = old })

	store := &memExpired{content: 0, orphan: 2}
	res, err := purgeChannel(context.Background(), store, "dtalk_group_msg_content", "dtalk_group_msg_relation", 1, 1, 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	if store.orphan != 0 || res.Deleted != 2 || res.More {
		t.Fatalf("result=%+v orphanLeft=%d", res, store.orphan)
	}
}

func TestCleanExpiredObjects(t *testing.T) {
	store := newMemObjects([]string{
		"chatList/picture/20261029/a.jpg",
		"chatList/picture/20261029/x_square_y.jpg",
		"chatList/picture/20261030/b.jpg",
		"chatList/video/20261029/v.mp4",
		"chatList/audio/20261028/a.m4a",
		"chatList/file/temp/a.bin",
		"chatList/avatar/20261029/a.jpg",
	})
	res, err := cleanExpiredObjects(context.Background(), store, mediaPrefixes, "20261030", 100)
	if err != nil {
		t.Fatal(err)
	}
	if res.More {
		t.Fatal("did not expect more")
	}
	if res.SkippedSquare != 1 {
		t.Fatalf("skipped square = %d, want 1", res.SkippedSquare)
	}
	if res.Removed != 3 {
		t.Fatalf("removed = %d, want 3", res.Removed)
	}
	for _, key := range []string{
		"chatList/picture/20261029/x_square_y.jpg",
		"chatList/picture/20261030/b.jpg",
		"chatList/file/temp/a.bin",
		"chatList/avatar/20261029/a.jpg",
	} {
		if !store.has(key) {
			t.Fatalf("kept key missing: %s", key)
		}
	}
	for _, key := range []string{
		"chatList/picture/20261029/a.jpg",
		"chatList/video/20261029/v.mp4",
		"chatList/audio/20261028/a.m4a",
	} {
		if store.has(key) {
			t.Fatalf("expired key still present: %s", key)
		}
	}
}

func TestCleanExpiredObjectsMaxObjects(t *testing.T) {
	store := newMemObjects([]string{
		"chatList/picture/20261028/a.jpg",
		"chatList/picture/20261029/b.jpg",
	})
	res, err := cleanExpiredObjects(context.Background(), store, mediaPrefixes, "20261030", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !res.More || res.Removed != 1 {
		t.Fatalf("result = %+v, want removed 1 and more", res)
	}
	if !store.has("chatList/picture/20261029/b.jpg") {
		t.Fatal("newer expired object should wait for the next round")
	}
	if store.has("chatList/picture/20261028/a.jpg") {
		t.Fatal("oldest object should be removed first")
	}
}

func snowflakeAt(ms int64) int64 {
	delta := ms - snowflakeEpoch
	if delta <= 0 {
		return 1
	}
	return delta << snowflakeTimeShift
}

type memExpired struct {
	content     int
	orphan      int64
	orphanCalls int
}

func (m *memExpired) DeleteExpiredContent(_ context.Context, _, _ string, _, _ int64, limit int) (int, error) {
	n := limit
	if m.content < n {
		n = m.content
	}
	m.content -= n
	return n, nil
}

func (m *memExpired) DeleteExpiredRelations(_ context.Context, _ string, _, _ int64, limit int) (int64, error) {
	m.orphanCalls++
	n := int64(limit)
	if m.orphan < n {
		n = m.orphan
	}
	m.orphan -= n
	return n, nil
}

type memObjects struct {
	keys map[string]struct{}
}

func newMemObjects(keys []string) *memObjects {
	m := &memObjects{keys: make(map[string]struct{}, len(keys))}
	for _, key := range keys {
		m.keys[key] = struct{}{}
	}
	return m
}

func (m *memObjects) has(key string) bool {
	_, ok := m.keys[key]
	return ok
}

func (m *memObjects) List(_ context.Context, prefix string, recursive bool) ([]string, error) {
	if recursive {
		out := make([]string, 0)
		for key := range m.keys {
			if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
				out = append(out, key)
			}
		}
		return out, nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for key := range m.keys {
		if len(key) < len(prefix) || key[:len(prefix)] != prefix {
			continue
		}
		rest := key[len(prefix):]
		i := indexByte(rest, '/')
		if i < 0 {
			continue
		}
		dir := prefix + rest[:i+1]
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		out = append(out, dir)
	}
	return out, nil
}

func (m *memObjects) Remove(_ context.Context, key string) error {
	delete(m.keys, key)
	return nil
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
