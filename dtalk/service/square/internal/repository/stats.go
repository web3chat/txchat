package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

type overviewTotalsScan struct {
	Posts    int64
	Comments int64
	Views    int64
	Likes    int64
}

type dayCountScan struct {
	Day string
	N   int64
}

func (s *Store) OverviewStats(ctx context.Context, from, until time.Time) (*entity.OverviewStats, error) {
	total, err := s.overviewTotals(ctx)
	if err != nil {
		return nil, err
	}
	posts, err := s.countByDay(ctx, `
SELECT ((created_at AT TIME ZONE 'Asia/Shanghai')::date)::text, COUNT(1)
FROM square_post
WHERE created_at >= $1 AND created_at < $2 AND status <> $3 AND status <> $4
GROUP BY 1`, from, until, entity.StatusDeleted, entity.StatusDraft)
	if err != nil {
		return nil, fmt.Errorf("count posts by day from=%s until=%s: %w", from.Format(time.RFC3339), until.Format(time.RFC3339), err)
	}
	comments, err := s.countByDay(ctx, `
SELECT ((created_at AT TIME ZONE 'Asia/Shanghai')::date)::text, COUNT(1)
FROM square_comment
WHERE created_at >= $1 AND created_at < $2 AND status <> $3
GROUP BY 1`, from, until, entity.StatusDeleted)
	if err != nil {
		return nil, fmt.Errorf("count comments by day from=%s until=%s: %w", from.Format(time.RFC3339), until.Format(time.RFC3339), err)
	}
	fromDay := from.Format("2006-01-02")
	untilDay := until.Format("2006-01-02")
	views, err := s.countByDay(ctx, `
SELECT viewed_on::text, COUNT(1)
FROM square_post_view
WHERE viewed_on >= $1::date AND viewed_on < $2::date
GROUP BY 1`, fromDay, untilDay)
	if err != nil {
		return nil, fmt.Errorf("count views by day from=%s until=%s: %w", from.Format(time.RFC3339), until.Format(time.RFC3339), err)
	}
	likes, err := s.countByDay(ctx, `
SELECT ((created_at AT TIME ZONE 'Asia/Shanghai')::date)::text, COUNT(1)
FROM square_post_like
WHERE created_at >= $1 AND created_at < $2
GROUP BY 1`, from, until)
	if err != nil {
		return nil, fmt.Errorf("count likes by day from=%s until=%s: %w", from.Format(time.RFC3339), until.Format(time.RFC3339), err)
	}
	last := until.AddDate(0, 0, -1)
	series := mergeDailySeries(from, last, posts, comments, views, likes)
	out := &entity.OverviewStats{Total: total, Series: series}
	if n := len(series); n > 0 {
		out.Today = countsOf(series[n-1])
	}
	if n := len(series); n > 1 {
		out.Yesterday = countsOf(series[n-2])
	}
	return out, nil
}

func (s *Store) AuthorPostSeries(ctx context.Context, from, until time.Time, addrs []string) ([]entity.StatDay, int64, error) {
	addrs = compactStrings(addrs)
	last := until.AddDate(0, 0, -1)
	empty := mergeDailySeries(from, last, nil, nil, nil, nil)
	if len(addrs) == 0 {
		return empty, 0, nil
	}
	ph := make([]string, 0, len(addrs))
	args := []any{from, until, entity.StatusDeleted, entity.StatusDraft}
	for _, addr := range addrs {
		args = append(args, addr)
		ph = append(ph, fmt.Sprintf("$%d", len(args)))
	}
	in := strings.Join(ph, ",")
	posts, err := s.countByDay(ctx, `
SELECT ((created_at AT TIME ZONE 'Asia/Shanghai')::date)::text, COUNT(1)
FROM square_post
WHERE created_at >= $1 AND created_at < $2 AND status <> $3 AND status <> $4
AND author_addr IN (`+in+`)
GROUP BY 1`, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("count author posts by day from=%s until=%s addrs=%v: %w", from.Format(time.RFC3339), until.Format(time.RFC3339), addrs, err)
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, `
SELECT COUNT(1) FROM square_post
WHERE created_at >= $1 AND created_at < $2 AND status <> $3 AND status <> $4
AND author_addr IN (`+in+`)`, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count author posts total from=%s until=%s addrs=%v: %w", from.Format(time.RFC3339), until.Format(time.RFC3339), addrs, err)
	}
	return mergeDailySeries(from, last, posts, nil, nil, nil), total, nil
}

func (s *Store) overviewTotals(ctx context.Context) (entity.StatCounts, error) {
	row := overviewTotalsScan{}
	err := s.db.QueryRowContext(ctx, `
SELECT
    (SELECT COUNT(1) FROM square_post WHERE status <> $1 AND status <> $2),
    (SELECT COUNT(1) FROM square_comment WHERE status <> $1),
    (SELECT COALESCE(SUM(view_count), 0) FROM square_post),
    (SELECT COUNT(1) FROM square_post_like)`, entity.StatusDeleted, entity.StatusDraft).
		Scan(&row.Posts, &row.Comments, &row.Views, &row.Likes)
	if err != nil {
		return entity.StatCounts{}, fmt.Errorf("overview totals: %w", err)
	}
	return entity.StatCounts{Posts: row.Posts, Comments: row.Comments, Views: row.Views, Likes: row.Likes}, nil
}

func (s *Store) countByDay(ctx context.Context, q string, args ...any) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int64)
	for rows.Next() {
		row := dayCountScan{}
		if err := rows.Scan(&row.Day, &row.N); err != nil {
			return nil, fmt.Errorf("scan day count: %w", err)
		}
		if row.Day == "" {
			continue
		}
		out[row.Day] = row.N
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func mergeDailySeries(from, last time.Time, posts, comments, views, likes map[string]int64) []entity.StatDay {
	from = truncateDate(from)
	last = truncateDate(last)
	if last.Before(from) {
		return []entity.StatDay{}
	}
	n := int(last.Sub(from).Hours()/24) + 1
	out := make([]entity.StatDay, 0, n)
	for d := from; !d.After(last); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		out = append(out, entity.StatDay{
			Date:     key,
			Posts:    posts[key],
			Comments: comments[key],
			Views:    views[key],
			Likes:    likes[key],
		})
	}
	return out
}

func truncateDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func countsOf(d entity.StatDay) entity.StatCounts {
	return entity.StatCounts{Posts: d.Posts, Comments: d.Comments, Views: d.Views, Likes: d.Likes}
}
