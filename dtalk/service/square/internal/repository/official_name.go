package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

func (s *Store) ListOfficialNames(ctx context.Context) ([]entity.OfficialName, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT n.id, n.name, COALESCE(n.avatar, ''), COALESCE(n.intro, ''), n.created_at,
       COALESCE(c.drafts, 0), COALESCE(c.posts, 0)
FROM square_official_name n
LEFT JOIN (
    SELECT author_name,
           COUNT(*) FILTER (WHERE status = $2) AS drafts,
           COUNT(*) FILTER (WHERE status IN ($3, $4)) AS posts
    FROM square_post
    WHERE author_addr = $1
    GROUP BY author_name
) c ON c.author_name = n.name
ORDER BY n.id`, entity.PlatformOfficialAddr, entity.StatusDraft, entity.StatusVisible, entity.StatusHidden)
	if err != nil {
		return nil, fmt.Errorf("list official names: %w", err)
	}
	defer rows.Close()
	list := make([]entity.OfficialName, 0)
	for rows.Next() {
		var n entity.OfficialName
		if err := rows.Scan(&n.ID, &n.Name, &n.Avatar, &n.Intro, &n.CreatedAt, &n.DraftCount, &n.PostCount); err != nil {
			return nil, fmt.Errorf("scan official name: %w", err)
		}
		list = append(list, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list official names: %w", err)
	}
	return list, nil
}

func (s *Store) GetOfficialName(ctx context.Context, id int64) (*entity.OfficialName, error) {
	n := &entity.OfficialName{}
	err := s.db.QueryRowContext(ctx, `
SELECT id, name, COALESCE(avatar, ''), COALESCE(intro, ''), created_at FROM square_official_name WHERE id=$1`, id).Scan(&n.ID, &n.Name, &n.Avatar, &n.Intro, &n.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get official name id=%d: %w", id, err)
	}
	return n, nil
}

func (s *Store) CountOfficialNames(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM square_official_name`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count official names: %w", err)
	}
	return n, nil
}

func (s *Store) OfficialNameExists(ctx context.Context, name string) (bool, error) {
	var ok bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM square_official_name WHERE name=$1)`, name).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("official name exists name=%s: %w", name, err)
	}
	return ok, nil
}

// OfficialPersonaNameTaken 表示名称已被其他公众号、申请或另一条官方名称占用。
// 平台官方号自己的目录名不算占用，方便把当前展示名收进名称列表。
func (s *Store) OfficialPersonaNameTaken(ctx context.Context, name string) (bool, error) {
	var taken bool
	err := s.db.QueryRowContext(ctx, `
SELECT (
    EXISTS (SELECT 1 FROM square_official_account WHERE name=$1 AND address<>$2)
    OR EXISTS (
        SELECT 1 FROM square_official_apply
        WHERE name=$1 AND status IN ($3, $4)
    )
    OR EXISTS (SELECT 1 FROM square_official_name WHERE name=$1)
)`, name, entity.PlatformOfficialAddr, entity.ApplyPending, entity.ApplyApproved).Scan(&taken)
	if err != nil {
		return false, fmt.Errorf("official persona name taken name=%s: %w", name, err)
	}
	return taken, nil
}

func (s *Store) InsertOfficialName(ctx context.Context, name, avatar, intro string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
INSERT INTO square_official_name (name, avatar, intro) VALUES ($1, $2, $3)
ON CONFLICT (name) DO NOTHING
RETURNING id`, name, avatar, intro).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrOfficialNameTaken
	}
	if err != nil {
		return 0, fmt.Errorf("insert official name name=%s: %w", name, err)
	}
	return id, nil
}

func (s *Store) DeleteOfficialName(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM square_official_name WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete official name id=%d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete official name id=%d rows: %w", id, err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) CountLiveOfficialPostsByName(ctx context.Context, name string) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(1) FROM square_post
WHERE author_addr=$1 AND author_name=$2 AND status<>$3`,
		entity.PlatformOfficialAddr, name, entity.StatusDeleted).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count official posts by name name=%s: %w", name, err)
	}
	return n, nil
}

func (s *Store) UpdateOfficialAccountName(ctx context.Context, addr, name string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE square_official_account SET name=$2, updated_at=NOW() WHERE address=$1`, addr, name)
	if err != nil {
		return fmt.Errorf("update official account name addr=%s: %w", addr, err)
	}
	return nil
}

func (s *Store) UpdateOfficialPersona(ctx context.Context, id int64, name, avatar, intro string) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE square_official_name SET name=$2, avatar=$3, intro=$4 WHERE id=$1`, id, name, avatar, intro)
	if err != nil {
		return fmt.Errorf("update official persona id=%d name=%s: %w", id, name, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update official persona id=%d rows: %w", id, err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) RenameOfficialPosts(ctx context.Context, from, to string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE square_post SET author_name=$2, updated_at=NOW()
WHERE author_addr=$1 AND author_name=$3 AND author_kind=$4`,
		entity.PlatformOfficialAddr, to, from, entity.KindOfficial)
	if err != nil {
		return fmt.Errorf("rename official posts from=%s to=%s: %w", from, to, err)
	}
	return nil
}

func (s *Store) FillOfficialNameAvatarIfEmpty(ctx context.Context, name, avatar string) error {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(avatar) == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE square_official_name SET avatar=$2 WHERE name=$1 AND avatar=''`, name, avatar)
	if err != nil {
		return fmt.Errorf("fill official name avatar name=%s: %w", name, err)
	}
	return nil
}

// OfficialPersonaFace 是读帖时按展示名带出的头像和简介。
type OfficialPersonaFace struct {
	Avatar string
	Intro  string
}

func (s *Store) OfficialNameAvatarMap(ctx context.Context) (map[string]OfficialPersonaFace, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, COALESCE(avatar, ''), COALESCE(intro, '') FROM square_official_name`)
	if err != nil {
		return nil, fmt.Errorf("official name avatar map: %w", err)
	}
	defer rows.Close()
	out := make(map[string]OfficialPersonaFace)
	for rows.Next() {
		var name string
		var face OfficialPersonaFace
		if err := rows.Scan(&name, &face.Avatar, &face.Intro); err != nil {
			return nil, fmt.Errorf("scan official name avatar: %w", err)
		}
		out[name] = face
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("official name avatar map: %w", err)
	}
	return out, nil
}

func (s *Store) FillOfficialNameIntroIfEmpty(ctx context.Context, name, intro string) error {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(intro) == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE square_official_name SET intro=$2 WHERE name=$1 AND intro=''`, name, intro)
	if err != nil {
		return fmt.Errorf("fill official name intro name=%s: %w", name, err)
	}
	return nil
}

func (s *Store) UpdateOfficialIntro(ctx context.Context, addr, intro string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE square_official_account SET intro=$2, updated_at=NOW() WHERE address=$1`, addr, intro)
	if err != nil {
		return fmt.Errorf("update official intro addr=%s: %w", addr, err)
	}
	return nil
}

func (s *Store) UpdateOfficialAvatar(ctx context.Context, addr, avatar string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE square_official_account SET avatar=$2, updated_at=NOW() WHERE address=$1`, addr, avatar)
	if err != nil {
		return fmt.Errorf("update official avatar addr=%s: %w", addr, err)
	}
	return nil
}
