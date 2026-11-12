package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

func (s *Store) HitRate(ctx context.Context, addr, action string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO square_rate_hit (addr, action, created_at) VALUES ($1,$2,NOW())`, addr, action)
	if err != nil {
		return fmt.Errorf("rate hit addr=%s action=%s: %w", addr, action, err)
	}
	return nil
}

func (s *Store) CountRate(ctx context.Context, addr, action string, since time.Time) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(1) FROM square_rate_hit WHERE addr=$1 AND action=$2 AND created_at>=$3`, addr, action, since).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count rate addr=%s action=%s: %w", addr, action, err)
	}
	return n, nil
}

func (s *Store) InsertModeration(ctx context.Context, m *entity.Moderation) error {
	const q = `
INSERT INTO square_content_moderation (target_type, target_id, verdict, categories, reason, model, latency_ms, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,NOW()) RETURNING id, created_at`
	err := s.db.QueryRowContext(ctx, q, m.TargetType, m.TargetID, m.Verdict, m.Categories, m.Reason, m.Model, m.LatencyMS).
		Scan(&m.ID, &m.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert moderation type=%s id=%d: %w", m.TargetType, m.TargetID, err)
	}
	return nil
}

func (s *Store) LatestModeration(ctx context.Context, targetType string, targetID int64) (*entity.Moderation, error) {
	const q = `
SELECT id, target_type, target_id, verdict, categories, reason, model, latency_ms,
       overridden_by, overridden_at, final_verdict, created_at
FROM square_content_moderation WHERE target_type=$1 AND target_id=$2
ORDER BY id DESC LIMIT 1`
	m := &entity.Moderation{}
	var overriddenBy sql.NullString
	var overriddenAt sql.NullTime
	var final sql.NullString
	err := s.db.QueryRowContext(ctx, q, targetType, targetID).Scan(
		&m.ID, &m.TargetType, &m.TargetID, &m.Verdict, &m.Categories, &m.Reason, &m.Model, &m.LatencyMS,
		&overriddenBy, &overriddenAt, &final, &m.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("latest moderation type=%s id=%d: %w", targetType, targetID, err)
	}
	if overriddenBy.Valid {
		v := overriddenBy.String
		m.OverriddenBy = &v
	}
	if overriddenAt.Valid {
		t := overriddenAt.Time
		m.OverriddenAt = &t
	}
	if final.Valid {
		v := final.String
		m.FinalVerdict = &v
	}
	return m, nil
}

func (s *Store) LatestModerationMap(ctx context.Context, targetType string, ids []int64) (map[int64]*entity.Moderation, error) {
	out := make(map[int64]*entity.Moderation)
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(ids)+1)
	args = append(args, targetType)
	ph := make([]string, 0, len(ids))
	for i, id := range ids {
		args = append(args, id)
		ph = append(ph, fmt.Sprintf("$%d", i+2))
	}
	q := `
SELECT DISTINCT ON (target_id)
       id, target_type, target_id, verdict, categories, reason, model, latency_ms,
       overridden_by, overridden_at, final_verdict, created_at
FROM square_content_moderation
WHERE target_type=$1 AND target_id IN (` + strings.Join(ph, ",") + `)
ORDER BY target_id, id DESC`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("latest moderation map type=%s: %w", targetType, err)
	}
	defer rows.Close()
	for rows.Next() {
		m, err := scanModeration(rows)
		if err != nil {
			return nil, err
		}
		out[m.TargetID] = m
	}
	return out, rows.Err()
}

func scanModeration(rows *sql.Rows) (*entity.Moderation, error) {
	m := &entity.Moderation{}
	var overriddenBy sql.NullString
	var overriddenAt sql.NullTime
	var final sql.NullString
	if err := rows.Scan(
		&m.ID, &m.TargetType, &m.TargetID, &m.Verdict, &m.Categories, &m.Reason, &m.Model, &m.LatencyMS,
		&overriddenBy, &overriddenAt, &final, &m.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan moderation: %w", err)
	}
	if overriddenBy.Valid {
		v := overriddenBy.String
		m.OverriddenBy = &v
	}
	if overriddenAt.Valid {
		t := overriddenAt.Time
		m.OverriddenAt = &t
	}
	if final.Valid {
		v := final.String
		m.FinalVerdict = &v
	}
	return m, nil
}

func (s *Store) OverrideModeration(ctx context.Context, id int64, by, verdict string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE square_content_moderation SET overridden_by=$2, overridden_at=NOW(), final_verdict=$3 WHERE id=$1`, id, by, verdict)
	if err != nil {
		return fmt.Errorf("override moderation id=%d: %w", id, err)
	}
	return nil
}
