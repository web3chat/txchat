package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

type AuthorRankFilter struct {
	Order       string
	From        *time.Time
	AuthorAddrs []string
	AuthorNames []string
}

type accountStatScan struct {
	Address           string
	AuthorName        string
	IsOfficial        bool
	OfficialStatus    int16
	PostCount         int64
	FollowerCount     int64
	FollowingCount    int64
	Avatar            string
	Intro             string
	Gold              bool
	OfficialBanReason string
	ReportCount       int64
	UserBanned        bool
	UserBanReason     string
	JoinedAt          sql.NullTime
}

func (a accountStatScan) toAccount() entity.AccountStat {
	out := entity.AccountStat{
		Address: a.Address, AuthorName: a.AuthorName, IsOfficial: a.IsOfficial, OfficialStatus: a.OfficialStatus,
		PostCount: a.PostCount, FollowerCount: a.FollowerCount, FollowingCount: a.FollowingCount,
		Avatar: a.Avatar, Intro: a.Intro, Gold: a.Gold, OfficialBanReason: a.OfficialBanReason,
		ReportCount: a.ReportCount, UserBanned: a.UserBanned, UserBanReason: a.UserBanReason,
	}
	if a.JoinedAt.Valid {
		t := a.JoinedAt.Time
		out.JoinedAt = &t
	}
	return out
}

type authorRankScan struct {
	Address      string
	AuthorName   string
	AuthorKind   int16
	IsOfficial   bool
	PostCount    int64
	LikeCount    int64
	ViewCount    int64
	CommentCount int64
}

func (s *Store) ListAccounts(ctx context.Context, addrs, names []string, page entity.PageQuery) ([]entity.AccountStat, int64, error) {
	addrs = compactStrings(addrs)
	names = compactStrings(names)
	page = page.Normalize()
	base, candArgs := accountCandidatesSQL(addrs, names)
	return s.listAccountsQuery(ctx, base, candArgs, page)
}

func (s *Store) listAccountsQuery(ctx context.Context, candidates string, candArgs []any, page entity.PageQuery) ([]entity.AccountStat, int64, error) {
	args := append([]any{}, candArgs...)
	args = append(args, entity.StatusDeleted, entity.StatusDraft, entity.TargetPost, page.PageSize, page.Offset())
	delPH := len(candArgs) + 1
	draftPH := len(candArgs) + 2
	reportPH := len(candArgs) + 3
	limPH := len(candArgs) + 4
	offPH := len(candArgs) + 5
	q := fmt.Sprintf(`
SELECT c.address,
       COALESCE(oa.name, nm.author_name, c.address),
       oa.address IS NOT NULL,
       COALESCE(oa.status, 0),
       COALESCE(pc.n, 0),
       COALESCE(fc.n, 0),
       COALESCE(fg.n, 0),
       COALESCE(oa.avatar, ''),
       COALESCE(oa.intro, ''),
       COALESCE(oa.gold, false),
       COALESCE(oa.ban_reason, ''),
       COALESCE(rc.n, 0),
       ub.address IS NOT NULL,
       COALESCE(ub.reason, ''),
       oa.created_at
FROM (%s) c
LEFT JOIN square_official_account oa ON oa.address = c.address
LEFT JOIN square_user_ban ub ON ub.address = c.address
LEFT JOIN LATERAL (
    SELECT author_name FROM square_post WHERE author_addr = c.address ORDER BY created_at DESC LIMIT 1
) nm ON true
LEFT JOIN (
    SELECT author_addr, COUNT(1) AS n FROM square_post
    WHERE status <> $%d AND status <> $%d
    GROUP BY author_addr
) pc ON pc.author_addr = c.address
LEFT JOIN (
    SELECT target_addr, COUNT(1) AS n FROM square_follow GROUP BY target_addr
) fc ON fc.target_addr = c.address
LEFT JOIN (
    SELECT follower_addr, COUNT(1) AS n FROM square_follow GROUP BY follower_addr
) fg ON fg.follower_addr = c.address
LEFT JOIN (
    SELECT p.author_addr, COUNT(1) AS n
    FROM square_report r
    JOIN square_post p ON p.id = r.target_id AND r.target_type = $%d
    GROUP BY p.author_addr
) rc ON rc.author_addr = c.address
ORDER BY COALESCE(fc.n, 0) DESC, COALESCE(pc.n, 0) DESC, c.address
LIMIT $%d OFFSET $%d`, candidates, delPH, draftPH, reportPH, limPH, offPH)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()
	list := make([]entity.AccountStat, 0)
	for rows.Next() {
		row := accountStatScan{}
		if err := rows.Scan(
			&row.Address, &row.AuthorName, &row.IsOfficial, &row.OfficialStatus,
			&row.PostCount, &row.FollowerCount, &row.FollowingCount,
			&row.Avatar, &row.Intro, &row.Gold, &row.OfficialBanReason, &row.ReportCount,
			&row.UserBanned, &row.UserBanReason, &row.JoinedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan account: %w", err)
		}
		list = append(list, row.toAccount())
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM (`+candidates+`) c`, candArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count accounts: %w", err)
	}
	return list, total, nil
}

func accountCandidatesSQL(addrs, names []string) (string, []any) {
	args := []any{}
	if len(addrs) > 0 {
		ph := make([]string, 0, len(addrs))
		for _, addr := range addrs {
			args = append(args, addr)
			ph = append(ph, fmt.Sprintf("$%d", len(args)))
		}
		sql := `SELECT DISTINCT address FROM (VALUES ` + joinValues(ph) + `) v(address)`
		if len(names) == 0 {
			return sql, args
		}
		or := nameLikeSQL("oa.name", names, &args)
		or2 := nameLikeSQL("p.author_name", names, &args)
		return `SELECT DISTINCT v.address FROM (VALUES ` + joinValues(ph) + `) v(address)
LEFT JOIN square_official_account oa ON oa.address = v.address
LEFT JOIN square_post p ON p.author_addr = v.address
WHERE (` + or + ` OR ` + or2 + `)`, args
	}
	if len(names) > 0 {
		orOA := nameLikeSQL("name", names, &args)
		orPost := nameLikeSQL("author_name", names, &args)
		return `SELECT address FROM square_official_account WHERE ` + orOA + `
UNION
SELECT DISTINCT author_addr FROM square_post WHERE ` + orPost, args
	}
	return `SELECT DISTINCT author_addr AS address FROM square_post
UNION
SELECT DISTINCT target_addr FROM square_follow
UNION
SELECT DISTINCT follower_addr FROM square_follow
UNION
SELECT address FROM square_official_account`, args
}

func (s *Store) ListAuthorRanks(ctx context.Context, f AuthorRankFilter, page entity.PageQuery) ([]entity.AuthorRank, int64, error) {
	page = page.Normalize()
	inner, args := authorRankInnerWhere(f)
	outer, args := authorRankOuterWhere(f.AuthorNames, args)
	order := sqlAuthorRankOrder(f.Order)
	args = append(args, page.PageSize, page.Offset())
	limPH := len(args) - 1
	offPH := len(args)
	q := fmt.Sprintf(`
SELECT g.author_addr,
       COALESCE(oa.name, nm.author_name, g.author_addr),
       g.author_kind,
       oa.address IS NOT NULL,
       g.post_count,
       g.like_count,
       g.view_count,
       g.comment_count
FROM (
    SELECT p.author_addr,
           MAX(p.author_kind) AS author_kind,
           COUNT(1) AS post_count,
           COALESCE(SUM(p.like_count), 0) AS like_count,
           COALESCE(SUM(p.view_count), 0) AS view_count,
           COALESCE(SUM(p.comment_count), 0) AS comment_count
    FROM square_post p
    WHERE %s
    GROUP BY p.author_addr
) g
LEFT JOIN square_official_account oa ON oa.address = g.author_addr
LEFT JOIN LATERAL (
    SELECT author_name FROM square_post WHERE author_addr = g.author_addr ORDER BY created_at DESC LIMIT 1
) nm ON true
%s
%s
LIMIT $%d OFFSET $%d`, inner, outer, order, limPH, offPH)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		from := ""
		if f.From != nil {
			from = f.From.Format(time.RFC3339)
		}
		return nil, 0, fmt.Errorf("list author ranks order=%s from=%s addrs=%v names=%v: %w", f.Order, from, f.AuthorAddrs, f.AuthorNames, err)
	}
	defer rows.Close()
	list := make([]entity.AuthorRank, 0)
	for rows.Next() {
		row := authorRankScan{}
		if err := rows.Scan(&row.Address, &row.AuthorName, &row.AuthorKind, &row.IsOfficial, &row.PostCount, &row.LikeCount, &row.ViewCount, &row.CommentCount); err != nil {
			return nil, 0, fmt.Errorf("scan author rank: %w", err)
		}
		list = append(list, entity.AuthorRank{
			Address: row.Address, AuthorName: row.AuthorName, AuthorKind: row.AuthorKind,
			IsOfficial: row.IsOfficial, PostCount: row.PostCount, LikeCount: row.LikeCount,
			ViewCount: row.ViewCount, CommentCount: row.CommentCount,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	countArgs := args[:len(args)-2]
	var total int64
	countQ := fmt.Sprintf(`
SELECT COUNT(1)
FROM (
    SELECT p.author_addr
    FROM square_post p
    WHERE %s
    GROUP BY p.author_addr
) g
LEFT JOIN square_official_account oa ON oa.address = g.author_addr
LEFT JOIN LATERAL (
    SELECT author_name FROM square_post WHERE author_addr = g.author_addr ORDER BY created_at DESC LIMIT 1
) nm ON true
%s`, inner, outer)
	if err := s.db.QueryRowContext(ctx, countQ, countArgs...).Scan(&total); err != nil {
		from := ""
		if f.From != nil {
			from = f.From.Format(time.RFC3339)
		}
		return nil, 0, fmt.Errorf("count author ranks order=%s from=%s addrs=%v names=%v: %w", f.Order, from, f.AuthorAddrs, f.AuthorNames, err)
	}
	return list, total, nil
}

func authorRankInnerWhere(f AuthorRankFilter) (string, []any) {
	args := []any{entity.StatusDeleted, entity.StatusDraft}
	where := "p.status <> $1 AND p.status <> $2"
	if f.From != nil {
		args = append(args, *f.From)
		where += fmt.Sprintf(" AND p.created_at >= $%d", len(args))
	}
	if addrs := compactStrings(f.AuthorAddrs); len(addrs) > 0 {
		ph := make([]string, 0, len(addrs))
		for _, addr := range addrs {
			args = append(args, addr)
			ph = append(ph, fmt.Sprintf("$%d", len(args)))
		}
		where += " AND p.author_addr IN (" + strings.Join(ph, ",") + ")"
	}
	return where, args
}

func authorRankOuterWhere(names []string, args []any) (string, []any) {
	names = compactStrings(names)
	if len(names) == 0 {
		return "", args
	}
	orOA := nameLikeSQL("oa.name", names, &args)
	orPost := nameLikeSQL("nm.author_name", names, &args)
	return " WHERE (" + orOA + " OR " + orPost + ")", args
}

func sqlAuthorRankOrder(order string) string {
	switch strings.ToLower(strings.TrimSpace(order)) {
	case "views", "view":
		return " ORDER BY g.view_count DESC, g.like_count DESC, g.post_count DESC, g.author_addr"
	default:
		return " ORDER BY g.like_count DESC, g.view_count DESC, g.post_count DESC, g.author_addr"
	}
}

func joinValues(ph []string) string {
	parts := make([]string, 0, len(ph))
	for _, p := range ph {
		parts = append(parts, "("+p+")")
	}
	return strings.Join(parts, ",")
}

func nameLikeSQL(col string, names []string, args *[]any) string {
	or := make([]string, 0, len(names))
	for _, name := range names {
		*args = append(*args, likeContains(name))
		or = append(or, fmt.Sprintf("%s ILIKE $%d ESCAPE '\\'", col, len(*args)))
	}
	return "(" + strings.Join(or, " OR ") + ")"
}
