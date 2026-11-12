package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

// officialReplyScan 与列表 SELECT 列一一对应。
type officialReplyScan struct {
	PostID           int64
	Title            string
	Intro            string
	Content          string
	PostStatus       int16
	ReplyCount       int64
	LatestCommentID  int64
	LatestAuthorName string
	LatestAuthorAddr string
	LatestContent    string
	LatestAt         time.Time
}

func (s *Store) ListOfficialPostReplies(ctx context.Context, page entity.PageQuery) ([]entity.OfficialPostReply, int64, error) {
	page = page.Normalize()
	const where = `
WHERE p.author_addr = $1
  AND p.status <> $2
  AND p.status <> $3
  AND EXISTS (
    SELECT 1 FROM square_comment c
    WHERE c.post_id = p.id AND c.status = $4 AND c.author_addr <> $1
  )`
	args := []any{entity.PlatformOfficialAddr, entity.StatusDeleted, entity.StatusDraft, entity.StatusVisible}
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM square_post p `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count official post replies author=%s: %w", entity.PlatformOfficialAddr, err)
	}
	q := `
SELECT p.id, p.title, p.intro, p.content, p.status, rc.n,
       lc.id, lc.author_name, lc.author_addr, lc.content, lc.created_at
FROM square_post p
INNER JOIN LATERAL (
    SELECT COUNT(1) AS n
    FROM square_comment c
    WHERE c.post_id = p.id AND c.status = $4 AND c.author_addr <> $1
) rc ON true
INNER JOIN LATERAL (
    SELECT id, author_name, author_addr, content, created_at
    FROM square_comment c
    WHERE c.post_id = p.id AND c.status = $4 AND c.author_addr <> $1
    ORDER BY c.created_at DESC, c.id DESC
    LIMIT 1
) lc ON true
` + where + `
ORDER BY lc.created_at DESC, p.id DESC
LIMIT $5 OFFSET $6`
	rows, err := s.db.QueryContext(ctx, q, append(args, page.PageSize, page.Offset())...)
	if err != nil {
		return nil, 0, fmt.Errorf("list official post replies author=%s page=%d: %w", entity.PlatformOfficialAddr, page.Page, err)
	}
	defer rows.Close()
	list := make([]entity.OfficialPostReply, 0)
	for rows.Next() {
		var row officialReplyScan
		if err = rows.Scan(
			&row.PostID, &row.Title, &row.Intro, &row.Content, &row.PostStatus, &row.ReplyCount,
			&row.LatestCommentID, &row.LatestAuthorName, &row.LatestAuthorAddr, &row.LatestContent, &row.LatestAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan official post reply: %w", err)
		}
		list = append(list, entity.OfficialPostReply{
			PostID: row.PostID, Title: row.Title, Intro: row.Intro, Content: row.Content,
			PostStatus: row.PostStatus, ReplyCount: row.ReplyCount,
			LatestCommentID: row.LatestCommentID, LatestAuthorName: row.LatestAuthorName,
			LatestAuthorAddr: row.LatestAuthorAddr, LatestContent: row.LatestContent, LatestAt: row.LatestAt,
		})
	}
	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate official post replies: %w", err)
	}
	return list, total, nil
}
