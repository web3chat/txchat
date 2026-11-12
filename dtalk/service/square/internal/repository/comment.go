package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

type CommentListFilter struct {
	PostID        int64
	AuthorAddr    string
	Statuses      []int16
	OnlyVisible   bool
	Verdicts      []string
	Order         string
	RootsOnly     bool
	ReplyParentID int64
}

func (s *Store) InsertComment(ctx context.Context, c *entity.Comment) (int64, error) {
	const q = `
INSERT INTO square_comment (
    post_id, author_addr, author_name, content, parent_id, reply_to_id, reply_to_addr, reply_to_name, status, created_at, updated_at
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW(),NOW())
RETURNING id, created_at, updated_at`
	err := s.db.QueryRowContext(ctx, q,
		c.PostID, c.AuthorAddr, c.AuthorName, c.Content, c.ParentID, c.ReplyToID, c.ReplyToAddr, c.ReplyToName, c.Status,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return 0, fmt.Errorf("insert comment: %w", err)
	}
	return c.ID, nil
}

func (s *Store) GetComment(ctx context.Context, id int64) (*entity.Comment, error) {
	c, err := scanComment(s.db.QueryRowContext(ctx, commentSelect+` WHERE id=$1`, id))
	if err != nil {
		return nil, fmt.Errorf("get comment id=%d: %w", id, err)
	}
	return c, nil
}

func (s *Store) UpdateCommentStatus(ctx context.Context, id int64, status int16) error {
	_, err := s.db.ExecContext(ctx, `UPDATE square_comment SET status=$2, updated_at=NOW() WHERE id=$1`, id, status)
	if err != nil {
		return fmt.Errorf("update comment status id=%d status=%d: %w", id, status, err)
	}
	return nil
}

func (s *Store) ListComments(ctx context.Context, f CommentListFilter, page entity.PageQuery) ([]entity.Comment, int64, error) {
	where, args := commentWhere(f)
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM square_comment `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count comments: %w", err)
	}
	page = page.Normalize()
	args = append(args, page.PageSize, page.Offset())
	q := commentSelect + " " + where + sqlOrderByCreatedAt(f.Order, "ASC") + fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	list, err := queryComments(ctx, s.db, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list comments: %w", err)
	}
	return list, total, nil
}

// ListThreadHeads 返回一级评论楼：可见楼，以及自己不可见但仍有可见回复的占位楼。
func (s *Store) ListThreadHeads(ctx context.Context, postID int64, page entity.PageQuery) ([]entity.Comment, int64, error) {
	const where = `
WHERE post_id=$1 AND parent_id=0 AND (
    status=$2 OR EXISTS (
        SELECT 1 FROM square_comment r
        WHERE r.parent_id = square_comment.id AND r.status=$2
    )
)`
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM square_comment `+where, postID, entity.StatusVisible).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count thread heads post=%d: %w", postID, err)
	}
	page = page.Normalize()
	q := commentSelect + where + ` ORDER BY created_at ASC, id ASC LIMIT $3 OFFSET $4`
	list, err := queryComments(ctx, s.db, q, postID, entity.StatusVisible, page.PageSize, page.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list thread heads post=%d: %w", postID, err)
	}
	return list, total, nil
}

// ListVisibleReplyPreview 返回某楼最早的若干条可见回复。
func (s *Store) ListVisibleReplyPreview(ctx context.Context, postID, parentID int64, limit int) ([]entity.Comment, error) {
	if limit < 1 {
		limit = 1
	}
	q := commentSelect + ` WHERE post_id=$1 AND parent_id=$2 AND status=$3 ORDER BY created_at ASC, id ASC LIMIT $4`
	list, err := queryComments(ctx, s.db, q, postID, parentID, entity.StatusVisible, limit)
	if err != nil {
		return nil, fmt.Errorf("list reply preview post=%d parent=%d: %w", postID, parentID, err)
	}
	return list, nil
}

const commentSelect = `
SELECT id, post_id, author_addr, author_name, content, parent_id, reply_to_id, reply_to_addr, reply_to_name, reply_count, status, created_at, updated_at,
       (SELECT COUNT(1) FROM square_comment f
        WHERE (
            (square_comment.parent_id = 0 AND f.post_id = square_comment.post_id AND f.parent_id = 0)
            OR (square_comment.parent_id > 0 AND f.parent_id = square_comment.parent_id)
          )
          AND (f.created_at < square_comment.created_at
               OR (f.created_at = square_comment.created_at AND f.id <= square_comment.id)))
FROM square_comment`

type commentScanner interface {
	Scan(dest ...any) error
}

type commentQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func queryComments(ctx context.Context, q commentQueryer, query string, args ...any) ([]entity.Comment, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]entity.Comment, 0)
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		if c != nil {
			list = append(list, *c)
		}
	}
	return list, rows.Err()
}

func scanComment(row commentScanner) (*entity.Comment, error) {
	c := &entity.Comment{}
	err := row.Scan(
		&c.ID, &c.PostID, &c.AuthorAddr, &c.AuthorName, &c.Content, &c.ParentID,
		&c.ReplyToID, &c.ReplyToAddr, &c.ReplyToName, &c.ReplyCount,
		&c.Status, &c.CreatedAt, &c.UpdatedAt, &c.Floor,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan comment: %w", err)
	}
	return c, nil
}

func commentWhere(f CommentListFilter) (string, []any) {
	w := []string{"1=1"}
	args := []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		w = append(w, fmt.Sprintf(cond, len(args)))
	}
	if f.PostID > 0 {
		add("post_id=$%d", f.PostID)
	}
	if f.AuthorAddr != "" {
		add("author_addr=$%d", f.AuthorAddr)
	}
	if f.RootsOnly {
		w = append(w, "parent_id=0")
	}
	if f.ReplyParentID > 0 {
		add("parent_id=$%d", f.ReplyParentID)
	}
	if f.OnlyVisible {
		add("status=$%d", entity.StatusVisible)
	} else if len(f.Statuses) > 0 {
		ph := make([]string, 0, len(f.Statuses))
		for _, st := range f.Statuses {
			args = append(args, st)
			ph = append(ph, fmt.Sprintf("$%d", len(args)))
		}
		w = append(w, "status IN ("+strings.Join(ph, ",")+")")
	}
	if len(f.Verdicts) > 0 {
		ph := make([]string, 0, len(f.Verdicts))
		for _, v := range f.Verdicts {
			args = append(args, v)
			ph = append(ph, fmt.Sprintf("$%d", len(args)))
		}
		w = append(w, `id IN (
			SELECT m.target_id FROM (
				SELECT DISTINCT ON (target_id) target_id, COALESCE(final_verdict, verdict) AS v
				FROM square_content_moderation
				WHERE target_type='comment'
				ORDER BY target_id, id DESC
			) m WHERE m.v IN (`+strings.Join(ph, ",")+`)
		)`)
	}
	return "WHERE " + strings.Join(w, " AND "), args
}
