package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

func (s *Store) GetUserBan(ctx context.Context, addr string) (*entity.UserBan, error) {
	const q = `SELECT address, reason, operator, created_at, updated_at FROM square_user_ban WHERE address=$1`
	b := &entity.UserBan{}
	err := s.db.QueryRowContext(ctx, q, addr).Scan(&b.Address, &b.Reason, &b.Operator, &b.CreatedAt, &b.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user ban addr=%s: %w", addr, err)
	}
	return b, nil
}

func (s *Store) UpsertUserBan(ctx context.Context, b *entity.UserBan) error {
	const q = `
INSERT INTO square_user_ban (address, reason, operator, created_at, updated_at)
VALUES ($1,$2,$3,NOW(),NOW())
ON CONFLICT (address) DO UPDATE SET
    reason=EXCLUDED.reason, operator=EXCLUDED.operator, updated_at=NOW()`
	_, err := s.db.ExecContext(ctx, q, b.Address, b.Reason, b.Operator)
	if err != nil {
		return fmt.Errorf("upsert user ban addr=%s: %w", b.Address, err)
	}
	return nil
}

func (s *Store) DeleteUserBan(ctx context.Context, addr string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM square_user_ban WHERE address=$1`, addr)
	if err != nil {
		return false, fmt.Errorf("delete user ban addr=%s: %w", addr, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("delete user ban rows addr=%s: %w", addr, err)
	}
	return n > 0, nil
}

func (s *Store) UserBanMap(ctx context.Context, addrs []string) (map[string]struct{}, error) {
	out := make(map[string]struct{}, len(addrs))
	uniq := uniqueNonEmpty(addrs)
	if len(uniq) == 0 {
		return out, nil
	}
	ph := make([]string, 0, len(uniq))
	args := make([]any, 0, len(uniq))
	for _, a := range uniq {
		args = append(args, a)
		ph = append(ph, fmt.Sprintf("$%d", len(args)))
	}
	q := `SELECT address FROM square_user_ban WHERE address IN (` + strings.Join(ph, ",") + `)`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("user ban map: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var addr string
		if err := rows.Scan(&addr); err != nil {
			return nil, err
		}
		out[addr] = struct{}{}
	}
	return out, rows.Err()
}

func (s *Store) ListUserBans(ctx context.Context, addrs []string, order string, page entity.PageQuery) ([]entity.UserBan, int64, error) {
	where := "WHERE 1=1"
	args := []any{}
	if uniq := uniqueNonEmpty(addrs); len(uniq) > 0 {
		ph := make([]string, 0, len(uniq))
		for _, a := range uniq {
			args = append(args, likeContains(a))
			ph = append(ph, fmt.Sprintf("address ILIKE $%d ESCAPE '\\'", len(args)))
		}
		where += " AND (" + strings.Join(ph, " OR ") + ")"
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM square_user_ban `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count user bans: %w", err)
	}
	page = page.Normalize()
	args = append(args, page.PageSize, page.Offset())
	q := userBanSelect + " " + where + banOrderBy(order) +
		fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list user bans: %w", err)
	}
	defer rows.Close()
	list := make([]entity.UserBan, 0)
	for rows.Next() {
		b := entity.UserBan{}
		if err := rows.Scan(&b.Address, &b.Reason, &b.Operator, &b.CreatedAt, &b.UpdatedAt, &b.AuthorName); err != nil {
			return nil, 0, err
		}
		list = append(list, b)
	}
	return list, total, rows.Err()
}

const userBanSelect = `
SELECT address, reason, operator, created_at, updated_at,
       COALESCE(
           (SELECT p.author_name FROM square_post p WHERE p.author_addr = square_user_ban.address ORDER BY p.created_at DESC LIMIT 1),
           (SELECT c.author_name FROM square_comment c WHERE c.author_addr = square_user_ban.address ORDER BY c.created_at DESC LIMIT 1),
           ''
       )
FROM square_user_ban`

func banOrderBy(order string) string {
	dir := "DESC"
	if strings.EqualFold(strings.TrimSpace(order), "asc") {
		dir = "ASC"
	}
	return " ORDER BY created_at " + dir + ", address " + dir
}

func (s *Store) HideCommentsByAuthor(ctx context.Context, addr string) error {
	// 同一行只更新一次：作者自己的楼和回复一起隐藏时，楼的 reply_count 与 status 写在同一条 UPDATE 里。
	const q = `
WITH targets AS (
    SELECT id, post_id, parent_id
    FROM square_comment
    WHERE author_addr=$1 AND status=$3
), reply_delta AS (
    SELECT parent_id AS id, COUNT(*)::bigint AS n
    FROM targets
    WHERE parent_id > 0
    GROUP BY parent_id
), upd_comments AS (
    UPDATE square_comment c
    SET status = CASE WHEN EXISTS (SELECT 1 FROM targets t WHERE t.id = c.id) THEN $2 ELSE c.status END,
        reply_count = GREATEST(c.reply_count - COALESCE((SELECT n FROM reply_delta d WHERE d.id = c.id), 0), 0),
        updated_at = NOW()
    WHERE c.id IN (SELECT id FROM targets) OR c.id IN (SELECT id FROM reply_delta)
    RETURNING c.id
), post_counts AS (
    SELECT post_id, COUNT(*)::bigint AS n FROM targets GROUP BY post_id
), upd_posts AS (
    UPDATE square_post p
    SET comment_count=GREATEST(p.comment_count - c.n, 0), updated_at=NOW()
    FROM post_counts c
    WHERE p.id=c.post_id
    RETURNING p.id
), upd_notices AS (
    UPDATE square_reply_notice n
    SET content=''
    FROM targets t
    WHERE n.comment_id=t.id
    RETURNING n.id
)
SELECT
    (SELECT COUNT(*) FROM targets),
    (SELECT COUNT(*) FROM upd_comments),
    (SELECT COUNT(*) FROM upd_posts),
    (SELECT COUNT(*) FROM upd_notices)`
	var targets, comments, posts, notices int64
	err := s.db.QueryRowContext(ctx, q, addr, entity.StatusHidden, entity.StatusVisible).Scan(&targets, &comments, &posts, &notices)
	if err != nil {
		return fmt.Errorf("hide comments by author=%s: %w", addr, err)
	}
	return nil
}

func uniqueNonEmpty(vals []string) []string {
	seen := make(map[string]struct{}, len(vals))
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
