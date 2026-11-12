package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

// ErrOfficialNameTaken 表示该名称已被现有公众号或待审/已通过申请占用。
var ErrOfficialNameTaken = errors.New("official name taken")

func (s *Store) GetOfficial(ctx context.Context, addr string) (*entity.OfficialAccount, error) {
	const q = `
SELECT address, name, avatar, intro, status, COALESCE(apply_id,0), banned_at, ban_reason, created_at, updated_at, gold, gold_at
FROM square_official_account WHERE address=$1`
	row := s.db.QueryRowContext(ctx, q, addr)
	oa := &entity.OfficialAccount{}
	banned, goldAt, err := scanOfficialBase(row.Scan, oa)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get official addr=%s: %w", addr, err)
	}
	applyOfficialTimes(oa, banned, goldAt)
	return oa, nil
}

type OfficialProfile struct {
	Avatar string
	Gold   bool
}

func (s *Store) OfficialAvatarMap(ctx context.Context, addrs []string) (map[string]string, error) {
	meta, err := s.OfficialProfileMap(ctx, addrs)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(meta))
	for addr, p := range meta {
		out[addr] = p.Avatar
	}
	return out, nil
}

func (s *Store) OfficialProfileMap(ctx context.Context, addrs []string) (map[string]OfficialProfile, error) {
	addrs = uniqueNonEmpty(addrs)
	out := make(map[string]OfficialProfile, len(addrs))
	if len(addrs) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(addrs))
	ph := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		args = append(args, addr)
		ph = append(ph, fmt.Sprintf("$%d", len(args)))
	}
	q := `SELECT address, COALESCE(avatar,''), gold FROM square_official_account WHERE address IN (` + strings.Join(ph, ",") + `)`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("official profile map addrs=%v: %w", addrs, err)
	}
	defer rows.Close()
	for rows.Next() {
		var addr, avatar string
		var gold bool
		if err := rows.Scan(&addr, &avatar, &gold); err != nil {
			return nil, fmt.Errorf("scan official profile: %w", err)
		}
		out[addr] = OfficialProfile{Avatar: avatar, Gold: gold}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("official profile map rows addrs=%v: %w", addrs, err)
	}
	return out, nil
}

func (s *Store) UpsertOfficial(ctx context.Context, oa *entity.OfficialAccount) error {
	const q = `
INSERT INTO square_official_account (address, name, avatar, intro, status, apply_id, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,NOW(),NOW())
ON CONFLICT (address) DO UPDATE SET
    name=EXCLUDED.name, avatar=EXCLUDED.avatar, intro=EXCLUDED.intro,
    status=EXCLUDED.status, apply_id=EXCLUDED.apply_id, banned_at=NULL, ban_reason='', updated_at=NOW()`
	_, err := s.db.ExecContext(ctx, q, oa.Address, oa.Name, oa.Avatar, oa.Intro, oa.Status, oa.ApplyID)
	if err != nil {
		return fmt.Errorf("upsert official addr=%s: %w", oa.Address, err)
	}
	return nil
}

func (s *Store) BanOfficial(ctx context.Context, addr, reason string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE square_official_account SET status=$2, banned_at=NOW(), ban_reason=$3, updated_at=NOW()
WHERE address=$1`, addr, entity.OAStatusBanned, reason)
	if err != nil {
		return fmt.Errorf("ban official addr=%s: %w", addr, err)
	}
	return nil
}

func (s *Store) RevokeOfficial(ctx context.Context, addr string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("revoke official begin addr=%s: %w", addr, err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `DELETE FROM square_official_account WHERE address=$1`, addr)
	if err != nil {
		return 0, fmt.Errorf("revoke official delete addr=%s: %w", addr, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("revoke official rows addr=%s: %w", addr, err)
	}
	if n == 0 {
		return 0, nil
	}
	postRes, err := tx.ExecContext(ctx, `
UPDATE square_post SET author_kind=$2, tag='', updated_at=NOW()
WHERE author_addr=$1 AND author_kind=$3`, addr, entity.KindPersonal, entity.KindOfficial)
	if err != nil {
		return 0, fmt.Errorf("revoke official demote posts addr=%s: %w", addr, err)
	}
	posts, err := postRes.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("revoke official demote rows addr=%s: %w", addr, err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("revoke official commit addr=%s: %w", addr, err)
	}
	return posts, nil
}

func (s *Store) UnbanOfficial(ctx context.Context, addr string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE square_official_account SET status=$2, banned_at=NULL, ban_reason='', updated_at=NOW()
WHERE address=$1`, addr, entity.OAStatusNormal)
	if err != nil {
		return fmt.Errorf("unban official addr=%s: %w", addr, err)
	}
	return nil
}

func (s *Store) SetOfficialGold(ctx context.Context, addr string, gold bool) error {
	var err error
	if gold {
		_, err = s.db.ExecContext(ctx, `
UPDATE square_official_account SET gold=TRUE, gold_at=NOW(), updated_at=NOW()
WHERE address=$1`, addr)
	} else {
		_, err = s.db.ExecContext(ctx, `
UPDATE square_official_account SET gold=FALSE, gold_at=NULL, updated_at=NOW()
WHERE address=$1`, addr)
	}
	if err != nil {
		return fmt.Errorf("set official gold addr=%s gold=%t: %w", addr, gold, err)
	}
	return nil
}

func (s *Store) UpdateOfficialProfile(ctx context.Context, addr, name, avatar string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE square_official_account SET name=$2, avatar=$3, updated_at=NOW()
WHERE address=$1`, addr, name, avatar)
	if err != nil {
		return fmt.Errorf("update official profile addr=%s: %w", addr, err)
	}
	return nil
}

func (s *Store) UpdateOfficialPostNames(ctx context.Context, addr, name string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE square_post SET author_name=$2, updated_at=NOW()
WHERE author_addr=$1 AND author_kind=$3`, addr, name, entity.KindOfficial)
	if err != nil {
		return fmt.Errorf("update official post names addr=%s: %w", addr, err)
	}
	return nil
}

type OfficialListFilter struct {
	Statuses []int16
	Golds    []bool
	Order    string
}

func (s *Store) ListOfficial(ctx context.Context, f OfficialListFilter, page entity.PageQuery) ([]entity.OfficialAccount, int64, error) {
	where, oWhere, args := officialWhere(f)
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM square_official_account `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	page = page.Normalize()
	base := len(args)
	lim := base + 1
	off := base + 2
	delPH := base + 3
	draftPH := base + 4
	reportPH := base + 5
	args = append(args, page.PageSize, page.Offset(), entity.StatusDeleted, entity.StatusDraft, entity.TargetPost)
	q := fmt.Sprintf(`
SELECT o.address, o.name, o.avatar, o.intro, o.status, COALESCE(o.apply_id,0), o.banned_at, o.ban_reason, o.created_at, o.updated_at, o.gold, o.gold_at,
       COALESCE(pc.n, 0), COALESCE(fc.n, 0), COALESCE(rc.n, 0)
FROM square_official_account o
LEFT JOIN (
    SELECT author_addr, COUNT(1) AS n FROM square_post
    WHERE status <> $%d AND status <> $%d
    GROUP BY author_addr
) pc ON pc.author_addr = o.address
LEFT JOIN (
    SELECT target_addr, COUNT(1) AS n FROM square_follow GROUP BY target_addr
) fc ON fc.target_addr = o.address
LEFT JOIN (
    SELECT p.author_addr, COUNT(1) AS n
    FROM square_report r
    JOIN square_post p ON p.id = r.target_id AND r.target_type = $%d
    GROUP BY p.author_addr
) rc ON rc.author_addr = o.address
%s%s LIMIT $%d OFFSET $%d`, delPH, draftPH, reportPH, oWhere, officialOrderBy(f.Order), lim, off)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list official order=%s: %w", f.Order, err)
	}
	defer rows.Close()
	list := make([]entity.OfficialAccount, 0)
	for rows.Next() {
		oa := entity.OfficialAccount{}
		var postCount, followerCount, reportCount int64
		banned, goldAt, err := scanOfficialBase(rows.Scan, &oa, &postCount, &followerCount, &reportCount)
		if err != nil {
			return nil, 0, err
		}
		oa.PostCount = postCount
		oa.FollowerCount = followerCount
		oa.ReportCount = reportCount
		applyOfficialTimes(&oa, banned, goldAt)
		list = append(list, oa)
	}
	return list, total, rows.Err()
}

func officialWhere(f OfficialListFilter) (where, oWhere string, args []any) {
	where = "WHERE 1=1"
	oWhere = "WHERE 1=1"
	args = []any{}
	if len(f.Statuses) > 0 {
		ph := make([]string, 0, len(f.Statuses))
		for _, st := range f.Statuses {
			args = append(args, st)
			ph = append(ph, fmt.Sprintf("$%d", len(args)))
		}
		in := " AND status IN (" + strings.Join(ph, ",") + ")"
		where += in
		oWhere += " AND o.status IN (" + strings.Join(ph, ",") + ")"
	}
	golds := uniqueBools(f.Golds)
	if len(golds) == 1 {
		args = append(args, golds[0])
		where += fmt.Sprintf(" AND gold=$%d", len(args))
		oWhere += fmt.Sprintf(" AND o.gold=$%d", len(args))
	}
	return where, oWhere, args
}

func officialOrderBy(order string) string {
	switch strings.ToLower(strings.TrimSpace(order)) {
	case "followers":
		return " ORDER BY COALESCE(fc.n, 0) DESC, COALESCE(pc.n, 0) DESC, o.address ASC"
	case "posts":
		return " ORDER BY COALESCE(pc.n, 0) DESC, COALESCE(fc.n, 0) DESC, o.address ASC"
	case "reports":
		return " ORDER BY COALESCE(rc.n, 0) DESC, COALESCE(pc.n, 0) DESC, o.address ASC"
	default:
		return " ORDER BY o.created_at DESC"
	}
}

func scanOfficialBase(scan func(dest ...any) error, oa *entity.OfficialAccount, extra ...any) (banned, goldAt sql.NullTime, err error) {
	dest := []any{
		&oa.Address, &oa.Name, &oa.Avatar, &oa.Intro, &oa.Status, &oa.ApplyID, &banned, &oa.BanReason,
		&oa.CreatedAt, &oa.UpdatedAt, &oa.Gold, &goldAt,
	}
	dest = append(dest, extra...)
	err = scan(dest...)
	return banned, goldAt, err
}

func applyOfficialTimes(oa *entity.OfficialAccount, banned, goldAt sql.NullTime) {
	if banned.Valid {
		t := banned.Time
		oa.BannedAt = &t
	}
	if goldAt.Valid {
		t := goldAt.Time
		oa.GoldAt = &t
	}
}

func uniqueBools(vals []bool) []bool {
	seen := map[bool]struct{}{}
	out := make([]bool, 0, 2)
	for _, v := range vals {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func (s *Store) OfficialNameTaken(ctx context.Context, name string, exceptApplyID int64) (bool, error) {
	return queryOfficialNameTaken(ctx, s.db, name, exceptApplyID, true)
}

func (s *Store) OfficialNameGranted(ctx context.Context, name string, exceptApplyID int64) (bool, error) {
	return queryOfficialNameTaken(ctx, s.db, name, exceptApplyID, false)
}

func (s *Store) InsertApply(ctx context.Context, a *entity.OfficialApply) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("insert apply begin addr=%s name=%s: %w", a.Address, a.Name, err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, officialNameLock(a.Name)); err != nil {
		return 0, fmt.Errorf("insert apply lock addr=%s name=%s: %w", a.Address, a.Name, err)
	}
	taken, err := officialNameUsed(ctx, tx, a.Name, 0, true)
	if err != nil {
		return 0, fmt.Errorf("insert apply name check addr=%s name=%s: %w", a.Address, a.Name, err)
	}
	if taken {
		return 0, ErrOfficialNameTaken
	}

	const q = `
INSERT INTO square_official_apply (address, name, avatar, intro, status, created_at)
VALUES ($1,$2,$3,$4,$5,NOW()) RETURNING id, created_at`
	err = tx.QueryRowContext(ctx, q, a.Address, a.Name, a.Avatar, a.Intro, a.Status).Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("insert apply addr=%s name=%s: %w", a.Address, a.Name, err)
	}
	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("insert apply commit addr=%s name=%s: %w", a.Address, a.Name, err)
	}
	return a.ID, nil
}

type nameQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func officialNameLock(name string) string {
	return "official-name:" + name
}

func queryOfficialNameTaken(ctx context.Context, q nameQueryer, name string, exceptApplyID int64, includePending bool) (bool, error) {
	taken, err := officialNameUsed(ctx, q, name, exceptApplyID, includePending)
	if err != nil {
		return false, fmt.Errorf("official name taken name=%s except=%d pending=%t: %w", name, exceptApplyID, includePending, err)
	}
	return taken, nil
}

func officialNameUsed(ctx context.Context, q nameQueryer, name string, exceptApplyID int64, includePending bool) (bool, error) {
	query := `
SELECT (
    EXISTS (SELECT 1 FROM square_official_account WHERE name = $1)
    OR EXISTS (
        SELECT 1 FROM square_official_apply
        WHERE name = $1 AND id <> $2 AND status = $3
    )
)`
	args := []any{name, exceptApplyID, entity.ApplyApproved}
	if includePending {
		query = `
SELECT (
    EXISTS (SELECT 1 FROM square_official_account WHERE name = $1)
    OR EXISTS (
        SELECT 1 FROM square_official_apply
        WHERE name = $1 AND id <> $2 AND status IN ($3, $4)
    )
)`
		args = append(args, entity.ApplyPending)
	}
	var taken bool
	err := q.QueryRowContext(ctx, query, args...).Scan(&taken)
	return taken, err
}

func (s *Store) GetPendingApply(ctx context.Context, addr string) (*entity.OfficialApply, error) {
	return s.getApplyBy(ctx, `SELECT id, address, name, avatar, intro, status, COALESCE(reviewer,''), review_remark, created_at, reviewed_at
FROM square_official_apply WHERE address=$1 AND status=$2 ORDER BY id DESC LIMIT 1`, addr, entity.ApplyPending)
}

func (s *Store) GetApply(ctx context.Context, id int64) (*entity.OfficialApply, error) {
	return s.getApplyBy(ctx, `SELECT id, address, name, avatar, intro, status, COALESCE(reviewer,''), review_remark, created_at, reviewed_at
FROM square_official_apply WHERE id=$1`, id)
}

func (s *Store) getApplyBy(ctx context.Context, q string, args ...any) (*entity.OfficialApply, error) {
	a := &entity.OfficialApply{}
	var reviewed sql.NullTime
	err := s.db.QueryRowContext(ctx, q, args...).Scan(
		&a.ID, &a.Address, &a.Name, &a.Avatar, &a.Intro, &a.Status, &a.Reviewer, &a.ReviewRemark, &a.CreatedAt, &reviewed,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get apply: %w", err)
	}
	if reviewed.Valid {
		t := reviewed.Time
		a.ReviewedAt = &t
	}
	return a, nil
}

func (s *Store) ReviewApply(ctx context.Context, id int64, status, reviewer, remark string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE square_official_apply SET status=$2, reviewer=$3, review_remark=$4, reviewed_at=NOW()
WHERE id=$1`, id, status, reviewer, remark)
	if err != nil {
		return fmt.Errorf("review apply id=%d status=%s: %w", id, status, err)
	}
	return nil
}

func (s *Store) ListApply(ctx context.Context, statuses []string, page entity.PageQuery) ([]entity.OfficialApply, int64, error) {
	where := "WHERE 1=1"
	args := []any{}
	if len(statuses) > 0 {
		ph := make([]string, 0, len(statuses))
		for _, st := range statuses {
			args = append(args, st)
			ph = append(ph, fmt.Sprintf("$%d", len(args)))
		}
		where += " AND status IN (" + strings.Join(ph, ",") + ")"
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM square_official_apply `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	page = page.Normalize()
	args = append(args, page.PageSize, page.Offset())
	q := `SELECT id, address, name, avatar, intro, status, COALESCE(reviewer,''), review_remark, created_at, reviewed_at
FROM square_official_apply ` + where + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list apply: %w", err)
	}
	defer rows.Close()
	list := make([]entity.OfficialApply, 0)
	for rows.Next() {
		a := entity.OfficialApply{}
		var reviewed sql.NullTime
		if err := rows.Scan(&a.ID, &a.Address, &a.Name, &a.Avatar, &a.Intro, &a.Status, &a.Reviewer, &a.ReviewRemark, &a.CreatedAt, &reviewed); err != nil {
			return nil, 0, err
		}
		if reviewed.Valid {
			t := reviewed.Time
			a.ReviewedAt = &t
		}
		list = append(list, a)
	}
	return list, total, rows.Err()
}

func (s *Store) LatestApply(ctx context.Context, addr string) (*entity.OfficialApply, error) {
	return s.getApplyBy(ctx, `SELECT id, address, name, avatar, intro, status, COALESCE(reviewer,''), review_remark, created_at, reviewed_at
FROM square_official_apply WHERE address=$1 ORDER BY id DESC LIMIT 1`, addr)
}
