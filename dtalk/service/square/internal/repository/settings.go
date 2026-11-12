package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

const (
	settingGateMode    = "gate_mode"
	settingGateMessage = "gate_message"
	settingAllowOA     = "allow_officials"
	settingModLevel    = "moderation_level"
)

func (s *Store) LoadSquareSettings(ctx context.Context) (entity.SquareSettings, error) {
	out := entity.DefaultSquareSettings()
	rows, err := s.db.QueryContext(ctx, `SELECT key, value, updated_at FROM square_setting`)
	if err != nil {
		return out, fmt.Errorf("load square settings: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key, val string
		var updated sql.NullTime
		if err := rows.Scan(&key, &val, &updated); err != nil {
			return out, fmt.Errorf("scan square setting: %w", err)
		}
		if updated.Valid && updated.Time.After(out.UpdatedAt) {
			out.UpdatedAt = updated.Time
		}
		switch key {
		case settingGateMode:
			out.GateMode = strings.TrimSpace(val)
		case settingGateMessage:
			out.GateMessage = val
		case settingAllowOA:
			out.AllowOfficials = val != "0" && strings.ToLower(val) != "false"
		case settingModLevel:
			out.ModerationLevel = strings.TrimSpace(val)
		}
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("iterate square settings: %w", err)
	}
	posters, err := s.ListPosterAllow(ctx)
	if err != nil {
		return out, err
	}
	out.Posters = posters
	return out, nil
}

func (s *Store) SaveSquareSettings(ctx context.Context, st entity.SquareSettings) error {
	allow := "0"
	if st.AllowOfficials {
		allow = "1"
	}
	pairs := [][2]string{
		{settingGateMode, st.GateMode},
		{settingGateMessage, st.GateMessage},
		{settingAllowOA, allow},
		{settingModLevel, st.ModerationLevel},
	}
	const q = `
INSERT INTO square_setting (key, value, updated_at) VALUES ($1,$2,NOW())
ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_at=NOW()`
	for _, p := range pairs {
		if _, err := s.db.ExecContext(ctx, q, p[0], p[1]); err != nil {
			return fmt.Errorf("save square setting key=%s: %w", p[0], err)
		}
	}
	return nil
}

func (s *Store) ListPosterAllow(ctx context.Context) ([]entity.PosterAllow, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT address, remark, operator, created_at
FROM square_poster_allow ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list poster allow: %w", err)
	}
	defer rows.Close()
	list := make([]entity.PosterAllow, 0)
	for rows.Next() {
		var p entity.PosterAllow
		if err := rows.Scan(&p.Address, &p.Remark, &p.Operator, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan poster allow: %w", err)
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (s *Store) UpsertPosterAllow(ctx context.Context, p entity.PosterAllow) error {
	const q = `
INSERT INTO square_poster_allow (address, remark, operator, created_at)
VALUES ($1,$2,$3,NOW())
ON CONFLICT (address) DO UPDATE SET remark=EXCLUDED.remark, operator=EXCLUDED.operator`
	_, err := s.db.ExecContext(ctx, q, p.Address, p.Remark, p.Operator)
	if err != nil {
		return fmt.Errorf("upsert poster allow addr=%s: %w", p.Address, err)
	}
	return nil
}

func (s *Store) DeletePosterAllow(ctx context.Context, addr string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM square_poster_allow WHERE address=$1`, addr)
	if err != nil {
		return false, fmt.Errorf("delete poster allow addr=%s: %w", addr, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("delete poster allow rows addr=%s: %w", addr, err)
	}
	return n > 0, nil
}

func (s *Store) PosterAllowed(ctx context.Context, addr string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM square_poster_allow WHERE address=$1`, addr).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("poster allowed addr=%s: %w", addr, err)
	}
	return n > 0, nil
}
