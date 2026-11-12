package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

// ReplyNoticeFilter Read: nil 全部，true 只看已读，false 只看可展示未读。
type ReplyNoticeFilter struct {
	Read *bool
}

func (s *Store) ListReplyNotices(ctx context.Context, recipient string, f ReplyNoticeFilter, page entity.PageQuery) ([]entity.ReplyNotice, int64, error) {
	where, args := replyNoticeWhere(recipient, f)
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) `+replyNoticeFrom+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count reply notices recipient=%s: %w", recipient, err)
	}
	page = page.Normalize()
	args = append(args, page.PageSize, page.Offset())
	q := replyNoticeSelect + where + fmt.Sprintf(" ORDER BY n.created_at DESC, n.id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list reply notices recipient=%s: %w", recipient, err)
	}
	defer rows.Close()
	list := make([]entity.ReplyNotice, 0)
	for rows.Next() {
		n, err := scanReplyNotice(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, *n)
	}
	return list, total, rows.Err()
}

func (s *Store) CountDisplayableUnread(ctx context.Context, recipient string) (int64, error) {
	unread := false
	where, args := replyNoticeWhere(recipient, ReplyNoticeFilter{Read: &unread})
	var n int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) `+replyNoticeFrom+where, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("count unread notices recipient=%s: %w", recipient, err)
	}
	return n, nil
}

func (s *Store) MarkReplyNoticesRead(ctx context.Context, recipient string, all bool, ids []int64) error {
	if all {
		_, err := s.db.ExecContext(ctx, `UPDATE square_reply_notice SET read_at=NOW() WHERE recipient=$1 AND read_at IS NULL`, recipient)
		if err != nil {
			return fmt.Errorf("mark all reply notices read recipient=%s: %w", recipient, err)
		}
		return nil
	}
	ids = positiveIDs(ids)
	if len(ids) == 0 {
		return nil
	}
	args := make([]any, 0, len(ids)+1)
	args = append(args, recipient)
	ph := make([]string, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
		ph = append(ph, fmt.Sprintf("$%d", len(args)))
	}
	q := `UPDATE square_reply_notice SET read_at=NOW() WHERE recipient=$1 AND read_at IS NULL AND id IN (` + strings.Join(ph, ",") + `)`
	if _, err := s.db.ExecContext(ctx, q, args...); err != nil {
		return fmt.Errorf("mark reply notices read recipient=%s ids=%v: %w", recipient, ids, err)
	}
	return nil
}

const replyNoticeFrom = `
FROM square_reply_notice n
LEFT JOIN square_post p ON p.id = n.post_id
LEFT JOIN square_comment c ON c.id = n.comment_id`

const replyNoticeSelect = `
SELECT n.id, n.recipient, n.actor_addr, n.actor_name, n.post_id, n.post_title, n.root_id, n.comment_id, n.reply_to_id, n.content, n.read_at, n.created_at,
       COALESCE(p.status, 0), COALESCE(c.status, 0)
` + replyNoticeFrom

func replyNoticeWhere(recipient string, f ReplyNoticeFilter) (string, []any) {
	w := []string{"n.recipient=$1"}
	args := []any{recipient}
	if f.Read != nil && *f.Read {
		w = append(w, "n.read_at IS NOT NULL")
	}
	if f.Read != nil && !*f.Read {
		w = append(w, "n.read_at IS NULL")
		w = append(w, fmt.Sprintf("p.status=$%d", len(args)+1))
		args = append(args, entity.StatusVisible)
		w = append(w, fmt.Sprintf("c.status=$%d", len(args)+1))
		args = append(args, entity.StatusVisible)
		w = append(w, "n.content <> ''")
	}
	return " WHERE " + strings.Join(w, " AND "), args
}

func scanReplyNotice(row interface{ Scan(dest ...any) error }) (*entity.ReplyNotice, error) {
	n := &entity.ReplyNotice{}
	var readAt sql.NullTime
	err := row.Scan(
		&n.ID, &n.Recipient, &n.ActorAddr, &n.ActorName, &n.PostID, &n.PostTitle, &n.RootID, &n.CommentID, &n.ReplyToID, &n.Content, &readAt, &n.CreatedAt,
		&n.PostStatus, &n.CommentStatus,
	)
	if err != nil {
		return nil, fmt.Errorf("scan reply notice: %w", err)
	}
	if readAt.Valid {
		t := readAt.Time
		n.ReadAt = &t
	}
	return n, nil
}

func positiveIDs(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
