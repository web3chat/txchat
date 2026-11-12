package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

// CommentVisInput 一次提交里改评论可见性、帖子计数、楼回复数和收件箱正文。
type CommentVisInput struct {
	Comment    *entity.Comment
	WasVisible bool
	NowStatus  int16
	PostAuthor string
	PostTitle  string
	Recipient  string
}

func (s *Store) ApplyCommentVisibility(ctx context.Context, in CommentVisInput) error {
	if in.Comment == nil || in.Comment.ID <= 0 {
		return fmt.Errorf("apply comment visibility: missing comment")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin comment visibility tx id=%d: %w", in.Comment.ID, err)
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, `UPDATE square_comment SET status=$2, updated_at=NOW() WHERE id=$1`, in.Comment.ID, in.NowStatus); err != nil {
		return fmt.Errorf("update comment status id=%d status=%d: %w", in.Comment.ID, in.NowStatus, err)
	}
	nowVisible := in.NowStatus == entity.StatusVisible
	if in.WasVisible != nowVisible {
		delta := int64(1)
		if in.WasVisible {
			delta = -1
		}
		if _, err = tx.ExecContext(ctx, `UPDATE square_post SET comment_count=GREATEST(comment_count+$2,0), updated_at=NOW() WHERE id=$1`, in.Comment.PostID, delta); err != nil {
			return fmt.Errorf("incr comment_count post=%d delta=%d: %w", in.Comment.PostID, delta, err)
		}
		if in.Comment.ParentID > 0 {
			if _, err = tx.ExecContext(ctx, `UPDATE square_comment SET reply_count=GREATEST(reply_count+$2,0), updated_at=NOW() WHERE id=$1`, in.Comment.ParentID, delta); err != nil {
				return fmt.Errorf("incr reply_count root=%d delta=%d: %w", in.Comment.ParentID, delta, err)
			}
		}
	}
	if !in.WasVisible && nowVisible {
		if err = upsertReplyNotice(ctx, tx, in); err != nil {
			return err
		}
	}
	if in.WasVisible && !nowVisible {
		if _, err = tx.ExecContext(ctx, `UPDATE square_reply_notice SET content='' WHERE comment_id=$1`, in.Comment.ID); err != nil {
			return fmt.Errorf("clear reply notice content comment=%d: %w", in.Comment.ID, err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit comment visibility id=%d: %w", in.Comment.ID, err)
	}
	return nil
}

func upsertReplyNotice(ctx context.Context, tx *sql.Tx, in CommentVisInput) error {
	if in.Recipient == "" {
		return nil
	}
	rootID := in.Comment.ID
	if in.Comment.ParentID > 0 {
		rootID = in.Comment.ParentID
	}
	const q = `
INSERT INTO square_reply_notice (
    recipient, actor_addr, actor_name, post_id, post_title, root_id, comment_id, reply_to_id, content, created_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW())
ON CONFLICT (comment_id, recipient) DO UPDATE
SET content=EXCLUDED.content, post_title=EXCLUDED.post_title, actor_name=EXCLUDED.actor_name`
	_, err := tx.ExecContext(ctx, q,
		in.Recipient, in.Comment.AuthorAddr, in.Comment.AuthorName, in.Comment.PostID, in.PostTitle,
		rootID, in.Comment.ID, in.Comment.ReplyToID, in.Comment.Content,
	)
	if err != nil {
		return fmt.Errorf("upsert reply notice comment=%d recipient=%s: %w", in.Comment.ID, in.Recipient, err)
	}
	return nil
}
