package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

func (s *Store) AddFollow(ctx context.Context, follower, target string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `
INSERT INTO square_follow (follower_addr, target_addr) VALUES ($1,$2)
ON CONFLICT DO NOTHING`, follower, target)
	if err != nil {
		return false, fmt.Errorf("insert follow follower=%s target=%s: %w", follower, target, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *Store) RemoveFollow(ctx context.Context, follower, target string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM square_follow WHERE follower_addr=$1 AND target_addr=$2`, follower, target)
	if err != nil {
		return false, fmt.Errorf("delete follow follower=%s target=%s: %w", follower, target, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// followKindSQL 用显式 smallint。不标注时，外层 COUNT 用不到这一列，
// PostgreSQL 会把 $2/$3 描述成 text，pgx 无法把 int16 编码进去。
const followKindSQL = `CASE WHEN oa.address IS NULL THEN $2::smallint ELSE $3::smallint END`

func (s *Store) ListFollows(ctx context.Context, follower string, page entity.PageQuery) ([]entity.FollowProfile, int64, error) {
	return s.listFollowSide(ctx, `
SELECT f.target_addr, COALESCE(oa.name,''), COALESCE(oa.avatar,''),
       `+followKindSQL+`, f.created_at
FROM square_follow f
LEFT JOIN square_official_account oa ON oa.address = f.target_addr
WHERE f.follower_addr=$1`, follower, page, "list follows follower="+follower)
}

func (s *Store) ListFollowers(ctx context.Context, target string, page entity.PageQuery) ([]entity.FollowProfile, int64, error) {
	return s.listFollowSide(ctx, `
SELECT f.follower_addr, COALESCE(oa.name,''), COALESCE(oa.avatar,''),
       `+followKindSQL+`, f.created_at
FROM square_follow f
LEFT JOIN square_official_account oa ON oa.address = f.follower_addr
WHERE f.target_addr=$1`, target, page, "list followers target="+target)
}

func (s *Store) listFollowSide(ctx context.Context, base string, addr string, page entity.PageQuery, logKey string) ([]entity.FollowProfile, int64, error) {
	countQ := "SELECT COUNT(1) FROM (" + base + ") t"
	var total int64
	if err := s.db.QueryRowContext(ctx, countQ, addr, entity.KindPersonal, entity.KindOfficial).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count %s: %w", logKey, err)
	}
	page = page.Normalize()
	q := base + " ORDER BY f.created_at DESC LIMIT $4 OFFSET $5"
	rows, err := s.db.QueryContext(ctx, q, addr, entity.KindPersonal, entity.KindOfficial, page.PageSize, page.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", logKey, err)
	}
	defer rows.Close()
	list := make([]entity.FollowProfile, 0)
	addrs := make([]string, 0)
	for rows.Next() {
		p := entity.FollowProfile{}
		if err := rows.Scan(&p.Addr, &p.Name, &p.Avatar, &p.Kind, &p.FollowedAt); err != nil {
			return nil, 0, fmt.Errorf("scan %s: %w", logKey, err)
		}
		list = append(list, p)
		addrs = append(addrs, p.Addr)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	names, err := s.LatestAuthorNameMap(ctx, addrs)
	if err != nil {
		return nil, 0, err
	}
	for i := range list {
		if strings.TrimSpace(list[i].Name) == "" {
			if n := names[list[i].Addr]; n != "" {
				list[i].Name = n
			} else {
				list[i].Name = list[i].Addr
			}
		}
	}
	return list, total, nil
}

func (s *Store) LatestAuthorNameMap(ctx context.Context, addrs []string) (map[string]string, error) {
	addrs = uniqueNonEmpty(addrs)
	out := make(map[string]string, len(addrs))
	if len(addrs) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(addrs))
	ph := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		args = append(args, addr)
		ph = append(ph, fmt.Sprintf("$%d", len(args)))
	}
	in := strings.Join(ph, ",")
	q := `
SELECT author_addr, author_name FROM (
    SELECT author_addr, author_name, created_at,
           ROW_NUMBER() OVER (PARTITION BY author_addr ORDER BY created_at DESC) AS rn
    FROM (
        SELECT author_addr, author_name, created_at FROM square_post WHERE author_addr IN (` + in + `)
        UNION ALL
        SELECT author_addr, author_name, created_at FROM square_comment WHERE author_addr IN (` + in + `)
    ) u
) x WHERE rn = 1`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("latest author names addrs=%v: %w", addrs, err)
	}
	defer rows.Close()
	for rows.Next() {
		var addr, name string
		if err := rows.Scan(&addr, &name); err != nil {
			return nil, fmt.Errorf("scan latest author name: %w", err)
		}
		out[addr] = name
	}
	return out, rows.Err()
}

func (s *Store) FollowedSet(ctx context.Context, follower string, targets []string) (map[string]struct{}, error) {
	out := make(map[string]struct{})
	follower = strings.TrimSpace(follower)
	targets = uniqueNonEmpty(targets)
	if follower == "" || len(targets) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(targets)+1)
	args = append(args, follower)
	ph := make([]string, 0, len(targets))
	for _, addr := range targets {
		args = append(args, addr)
		ph = append(ph, fmt.Sprintf("$%d", len(args)))
	}
	q := `SELECT target_addr FROM square_follow WHERE follower_addr=$1 AND target_addr IN (` + strings.Join(ph, ",") + `)`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("followed set follower=%s n=%d: %w", follower, len(targets), err)
	}
	defer rows.Close()
	for rows.Next() {
		var addr string
		if err := rows.Scan(&addr); err != nil {
			return nil, fmt.Errorf("scan followed set: %w", err)
		}
		out[addr] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("followed set rows follower=%s: %w", follower, err)
	}
	return out, nil
}

func (s *Store) CountFollowers(ctx context.Context, addr string) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM square_follow WHERE target_addr=$1`, addr).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count followers addr=%s: %w", addr, err)
	}
	return n, nil
}
