package repository

import (
	"context"
	"fmt"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

// PublishDueOfficialPosts 把到点的官方定时帖改为可见，并把自动删除的起点设为实际发布时间。
func (s *Store) PublishDueOfficialPosts(ctx context.Context, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 100
	}
	const q = `
UPDATE square_post
SET status = $1,
    created_at = NOW(),
    updated_at = NOW(),
    expire_at = CASE
        WHEN expire_after_sec > 0 THEN NOW() + expire_after_sec * INTERVAL '1 second'
        ELSE NULL
    END,
    publish_at = NULL
WHERE id IN (
    SELECT id FROM square_post
    WHERE author_addr = $2 AND status = $3 AND publish_at IS NOT NULL AND publish_at <= NOW()
    ORDER BY publish_at, id
    LIMIT $4
)
RETURNING id`
	rows, err := s.db.QueryContext(ctx, q, entity.StatusVisible, entity.PlatformOfficialAddr, entity.StatusDraft, limit)
	if err != nil {
		return nil, fmt.Errorf("publish due official posts: %w", err)
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan published official post: %w", err)
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate published official posts: %w", err)
	}
	return ids, nil
}

// ExpireDueOfficialPosts 软删已到自动删除时间的官方帖，并带出正文和图片以便清理 OSS。
func (s *Store) ExpireDueOfficialPosts(ctx context.Context, limit int) ([]entity.Post, error) {
	if limit <= 0 {
		limit = 100
	}
	const q = `
UPDATE square_post
SET status = $1, pinned_at = NULL, updated_at = NOW()
WHERE id IN (
    SELECT id FROM square_post
    WHERE author_addr = $2 AND status IN ($3, $4) AND expire_at IS NOT NULL AND expire_at <= NOW()
    ORDER BY expire_at, id
    LIMIT $5
)
RETURNING id, media_json, content`
	rows, err := s.db.QueryContext(ctx, q,
		entity.StatusDeleted, entity.PlatformOfficialAddr, entity.StatusVisible, entity.StatusHidden, limit)
	if err != nil {
		return nil, fmt.Errorf("expire due official posts: %w", err)
	}
	defer rows.Close()
	list := make([]entity.Post, 0)
	for rows.Next() {
		var p entity.Post
		if err = rows.Scan(&p.ID, &p.MediaJSON, &p.Content); err != nil {
			return nil, fmt.Errorf("scan expired official post: %w", err)
		}
		list = append(list, p)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expired official posts: %w", err)
	}
	return list, nil
}
