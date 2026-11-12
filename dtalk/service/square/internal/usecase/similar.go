package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

const (
	duplicateWindow    = 24 * time.Hour
	duplicateScanLimit = 40
	duplicateRatio     = 0.9
)

func comparablePostText(title, intro, content string) string {
	return strings.Join(strings.Fields(strings.Join([]string{title, intro, content}, "\n")), "")
}

func nearDuplicate(a, b string) bool {
	a = comparablePostText(a, "", "")
	b = comparablePostText(b, "", "")
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	ar, br := []rune(a), []rune(b)
	if len(ar) < len(br) {
		ar, br = br, ar
	}
	maxL := len(ar)
	if (maxL-len(br))*10 > maxL {
		return false
	}
	maxDist := int(float64(maxL) * (1 - duplicateRatio))
	_, ok := levenshteinAtMost(ar, br, maxDist)
	return ok
}

func levenshteinAtMost(a, b []rune, maxDist int) (int, bool) {
	if maxDist < 0 {
		return 0, false
	}
	n, m := len(a), len(b)
	if n-m > maxDist {
		return 0, false
	}
	prev := make([]int, m+1)
	cur := make([]int, m+1)
	for j := 0; j <= m; j++ {
		prev[j] = j
	}
	for i := 1; i <= n; i++ {
		cur[0] = i
		rowMin := cur[0]
		for j := 1; j <= m; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := cur[j-1] + 1
			sub := prev[j-1] + cost
			best := del
			if ins < best {
				best = ins
			}
			if sub < best {
				best = sub
			}
			cur[j] = best
			if best < rowMin {
				rowMin = best
			}
		}
		if rowMin > maxDist {
			return 0, false
		}
		prev, cur = cur, prev
	}
	if prev[m] > maxDist {
		return 0, false
	}
	return prev[m], true
}

func (s *Service) rejectNearDuplicate(ctx context.Context, addr, title, intro, content string) error {
	text := comparablePostText(title, intro, content)
	if text == "" {
		return nil
	}
	since := time.Now().Add(-duplicateWindow)
	rows, err := s.store.ListAuthorPostTexts(ctx, addr, since, duplicateScanLimit)
	if err != nil {
		s.log.Error("list recent posts for duplicate check failed", "addr", addr, "since", since.Format(time.RFC3339), "err", err)
		return xerror.Wrap(xerror.Internal, "内部错误", err)
	}
	if len(rows) == duplicateScanLimit {
		s.log.Warn("duplicate check window truncated", "addr", addr, "limit", duplicateScanLimit)
	}
	for _, row := range rows {
		prev := comparablePostText(row.Title, row.Intro, row.Content)
		if !nearDuplicate(text, prev) {
			continue
		}
		s.log.Warn("reject near duplicate post", "addr", addr, "newRunes", len([]rune(text)), "prevRunes", len([]rune(prev)))
		return xerror.New(xerror.DuplicatePost, "24小时内已发布过高度相似的内容")
	}
	return nil
}
