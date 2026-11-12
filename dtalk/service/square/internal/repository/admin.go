package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

func (s *Store) CountAdmins(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM square_admin`).Scan(&n)
	return n, err
}

func (s *Store) InsertAdmin(ctx context.Context, a *entity.Admin) error {
	const q = `INSERT INTO square_admin (username, password_hash, role, status, permissions, created_at) VALUES ($1,$2,$3,$4,$5,NOW()) RETURNING id, created_at`
	err := s.db.QueryRowContext(ctx, q, a.Username, a.PasswordHash, a.Role, a.Status, entity.FormatPerms(a.Permissions)).Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		return err
	}
	a.PermReady = true
	a.Permissions = entity.ParsePerms(entity.FormatPerms(a.Permissions))
	return nil
}

func (s *Store) GetAdminByUsername(ctx context.Context, username string) (*entity.Admin, error) {
	const q = `SELECT id, username, password_hash, role, status, created_at, permissions FROM square_admin WHERE username=$1`
	a, err := scanAdmin(s.db.QueryRowContext(ctx, q, username))
	if err != nil {
		return nil, fmt.Errorf("get admin user=%s: %w", username, err)
	}
	return a, nil
}

func (s *Store) GetAdminByID(ctx context.Context, id int64) (*entity.Admin, error) {
	const q = `SELECT id, username, password_hash, role, status, created_at, permissions FROM square_admin WHERE id=$1`
	a, err := scanAdmin(s.db.QueryRowContext(ctx, q, id))
	if err != nil {
		return nil, fmt.Errorf("get admin id=%d: %w", id, err)
	}
	return a, nil
}

func (s *Store) ListAdmins(ctx context.Context) ([]*entity.Admin, error) {
	const q = `SELECT id, username, password_hash, role, status, created_at, permissions FROM square_admin ORDER BY id`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list admins: %w", err)
	}
	defer rows.Close()
	var out []*entity.Admin
	for rows.Next() {
		a, err := scanAdmin(rows)
		if err != nil {
			return nil, fmt.Errorf("list admins scan: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list admins: %w", err)
	}
	return out, nil
}

func (s *Store) UpdateAdminPassword(ctx context.Context, id int64, hash string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE square_admin SET password_hash=$2 WHERE id=$1`, id, hash)
	if err != nil {
		return fmt.Errorf("update admin password id=%d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update admin password id=%d rows: %w", id, err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) UpdateAdminStatus(ctx context.Context, id int64, status int16) error {
	res, err := s.db.ExecContext(ctx, `UPDATE square_admin SET status=$2 WHERE id=$1`, id, status)
	if err != nil {
		return fmt.Errorf("update admin status id=%d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update admin status id=%d rows: %w", id, err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) UpdateAdminPermissions(ctx context.Context, id int64, perms string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE square_admin SET permissions=$2 WHERE id=$1`, id, perms)
	if err != nil {
		return fmt.Errorf("update admin permissions id=%d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update admin permissions id=%d rows: %w", id, err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type adminScanner interface {
	Scan(dest ...any) error
}

type adminRow struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         string
	Status       int16
	CreatedAt    time.Time
	Permissions  sql.NullString
}

func scanAdmin(sc adminScanner) (*entity.Admin, error) {
	var row adminRow
	err := sc.Scan(&row.ID, &row.Username, &row.PasswordHash, &row.Role, &row.Status, &row.CreatedAt, &row.Permissions)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a := &entity.Admin{
		ID:           row.ID,
		Username:     row.Username,
		PasswordHash: row.PasswordHash,
		Role:         row.Role,
		Status:       row.Status,
		CreatedAt:    row.CreatedAt,
		PermReady:    row.Permissions.Valid,
	}
	if row.Permissions.Valid {
		a.Permissions = entity.ParsePerms(row.Permissions.String)
	}
	return a, nil
}

func (s *Store) InsertCaptcha(ctx context.Context, id, hash string, exp time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO square_admin_captcha (id, code_hash, expires_at) VALUES ($1,$2,$3)`, id, hash, exp)
	if err != nil {
		return fmt.Errorf("insert captcha: %w", err)
	}
	return nil
}

func (s *Store) TakeCaptcha(ctx context.Context, id string) (hash string, exp time.Time, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT code_hash, expires_at FROM square_admin_captcha WHERE id=$1`, id).Scan(&hash, &exp)
	if err == sql.ErrNoRows {
		return "", time.Time{}, nil
	}
	if err != nil {
		return "", time.Time{}, fmt.Errorf("take captcha id=%s: %w", id, err)
	}
	if _, delErr := s.db.ExecContext(ctx, `DELETE FROM square_admin_captcha WHERE id=$1`, id); delErr != nil {
		return "", time.Time{}, delErr
	}
	return hash, exp, nil
}

type LoginGuard struct {
	FailCount   int
	LockedUntil sql.NullTime
}

func (s *Store) GetGuard(ctx context.Context, key string) (*LoginGuard, error) {
	g := &LoginGuard{}
	err := s.db.QueryRowContext(ctx, `SELECT fail_count, locked_until FROM square_admin_login_guard WHERE guard_key=$1`, key).
		Scan(&g.FailCount, &g.LockedUntil)
	if err == sql.ErrNoRows {
		return &LoginGuard{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get guard key=%s: %w", key, err)
	}
	return g, nil
}

func (s *Store) SaveGuard(ctx context.Context, key string, failCount int, lockedUntil *time.Time) error {
	const q = `
INSERT INTO square_admin_login_guard (guard_key, fail_count, locked_until, updated_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (guard_key) DO UPDATE SET
    fail_count = EXCLUDED.fail_count,
    locked_until = EXCLUDED.locked_until,
    updated_at = NOW()`
	_, err := s.db.ExecContext(ctx, q, key, failCount, lockedUntil)
	if err != nil {
		return fmt.Errorf("save guard key=%s fails=%d: %w", key, failCount, err)
	}
	return nil
}

func (s *Store) ClearGuard(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM square_admin_login_guard WHERE guard_key=$1`, key)
	return err
}

type LoginFailIP struct {
	IP     string
	Fails  int64
	LastAt time.Time
}

func (s *Store) ListLoginFailIPs(ctx context.Context, since time.Time, minFails, limit int) ([]LoginFailIP, error) {
	if minFails < 1 {
		minFails = 1
	}
	if limit < 1 {
		limit = 8
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT client_ip, COUNT(1), MAX(created_at)
FROM square_admin_audit
WHERE action = 'login_fail' AND client_ip <> '' AND created_at >= $1
GROUP BY client_ip
HAVING COUNT(1) >= $2
ORDER BY COUNT(1) DESC, MAX(created_at) DESC
LIMIT $3`, since, minFails, limit)
	if err != nil {
		return nil, fmt.Errorf("list login fail ips since=%s min=%d: %w", since.Format(time.RFC3339), minFails, err)
	}
	defer rows.Close()
	list := make([]LoginFailIP, 0)
	for rows.Next() {
		var row LoginFailIP
		if err = rows.Scan(&row.IP, &row.Fails, &row.LastAt); err != nil {
			return nil, fmt.Errorf("scan login fail ip: %w", err)
		}
		list = append(list, row)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("list login fail ips: %w", err)
	}
	return list, nil
}

type AdminSession struct {
	ID        string
	AdminID   int64
	Username  string
	ClientIP  string
	ExpiresAt time.Time
}

type ActiveLoginIP struct {
	Username string
	IP       string
	LastSeen time.Time
}

func (s *Store) InsertAdminSession(ctx context.Context, sess AdminSession) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM square_admin_session WHERE expires_at < NOW() - INTERVAL '7 days'`); err != nil {
		return fmt.Errorf("prune admin session: %w", err)
	}
	const q = `
INSERT INTO square_admin_session (id, admin_id, username, client_ip, expires_at, last_seen, created_at)
VALUES ($1,$2,$3,$4,$5,NOW(),NOW())`
	_, err := s.db.ExecContext(ctx, q, sess.ID, sess.AdminID, sess.Username, sess.ClientIP, sess.ExpiresAt)
	if err != nil {
		return fmt.Errorf("insert admin session user=%s ip=%s: %w", sess.Username, sess.ClientIP, err)
	}
	return nil
}

func (s *Store) TouchAdminSession(ctx context.Context, id string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `
UPDATE square_admin_session
SET last_seen = NOW()
WHERE id = $1 AND revoked_at IS NULL AND expires_at > NOW()`, id)
	if err != nil {
		return false, fmt.Errorf("touch admin session id=%s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("touch admin session rows id=%s: %w", id, err)
	}
	return n > 0, nil
}

func (s *Store) RevokeAdminSession(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE square_admin_session SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("revoke admin session id=%s: %w", id, err)
	}
	return nil
}

func (s *Store) ListSharedLoginIPs(ctx context.Context, now time.Time) ([]ActiveLoginIP, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT username, client_ip, MAX(last_seen)
FROM square_admin_session
WHERE revoked_at IS NULL AND expires_at > $1 AND client_ip <> ''
  AND username IN (
    SELECT username FROM square_admin_session
    WHERE revoked_at IS NULL AND expires_at > $1 AND client_ip <> ''
    GROUP BY username
    HAVING COUNT(DISTINCT client_ip) >= 2
  )
GROUP BY username, client_ip
ORDER BY username, MAX(last_seen) DESC`, now)
	if err != nil {
		return nil, fmt.Errorf("list shared login ips at=%s: %w", now.Format(time.RFC3339), err)
	}
	defer rows.Close()
	list := make([]ActiveLoginIP, 0)
	for rows.Next() {
		var row ActiveLoginIP
		if err = rows.Scan(&row.Username, &row.IP, &row.LastSeen); err != nil {
			return nil, fmt.Errorf("scan shared login ip: %w", err)
		}
		list = append(list, row)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("list shared login ips: %w", err)
	}
	return list, nil
}

type ActiveSession struct {
	ID       string
	Username string
	IP       string
	LastSeen time.Time
}

func (s *Store) ListMultiLoginSessions(ctx context.Context, now time.Time) ([]ActiveSession, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, username, client_ip, last_seen
FROM square_admin_session
WHERE revoked_at IS NULL AND expires_at > $1 AND username <> ''
  AND username IN (
    SELECT username FROM square_admin_session
    WHERE revoked_at IS NULL AND expires_at > $1 AND username <> ''
    GROUP BY username
    HAVING COUNT(1) >= 2
  )
ORDER BY username, last_seen DESC`, now)
	if err != nil {
		return nil, fmt.Errorf("list multi login sessions at=%s: %w", now.Format(time.RFC3339), err)
	}
	defer rows.Close()
	list := make([]ActiveSession, 0)
	for rows.Next() {
		var row ActiveSession
		if err = rows.Scan(&row.ID, &row.Username, &row.IP, &row.LastSeen); err != nil {
			return nil, fmt.Errorf("scan multi login session: %w", err)
		}
		list = append(list, row)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("list multi login sessions: %w", err)
	}
	return list, nil
}

func (s *Store) CountLoginHits(ctx context.Context, ip string, since time.Time) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(1) FROM square_admin_audit
WHERE action='login_attempt' AND client_ip=$1 AND created_at>=$2`, ip, since).Scan(&n)
	return n, err
}

func (s *Store) InsertAudit(ctx context.Context, a *entity.Audit) error {
	const q = `
INSERT INTO square_admin_audit (admin_id, username, action, target_type, target_id, extra, client_ip, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())`
	_, err := s.db.ExecContext(ctx, q, a.AdminID, a.Username, a.Action, a.TargetType, a.TargetID, a.Extra, a.ClientIP)
	if err != nil {
		return fmt.Errorf("insert audit action=%s: %w", a.Action, err)
	}
	return nil
}

type AuditFilter struct {
	Actions     []string
	TargetTypes []string
	TargetID    string
}

func (s *Store) ListAudit(ctx context.Context, f AuditFilter, page entity.PageQuery) ([]entity.Audit, int64, error) {
	where := "WHERE 1=1"
	args := []any{}
	if len(f.Actions) > 0 {
		ph := make([]string, 0, len(f.Actions))
		for _, a := range f.Actions {
			args = append(args, a)
			ph = append(ph, fmt.Sprintf("$%d", len(args)))
		}
		where += " AND action IN (" + strings.Join(ph, ",") + ")"
	}
	if len(f.TargetTypes) > 0 {
		ph := make([]string, 0, len(f.TargetTypes))
		for _, t := range f.TargetTypes {
			args = append(args, t)
			ph = append(ph, fmt.Sprintf("$%d", len(args)))
		}
		where += " AND target_type IN (" + strings.Join(ph, ",") + ")"
	}
	if f.TargetID != "" {
		args = append(args, f.TargetID)
		where += fmt.Sprintf(" AND target_id=$%d", len(args))
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM square_admin_audit `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit: %w", err)
	}
	page = page.Normalize()
	args = append(args, page.PageSize, page.Offset())
	rows, err := s.db.QueryContext(ctx, `
SELECT id, admin_id, username, action, target_type, target_id, extra, client_ip, created_at
FROM square_admin_audit `+where+` ORDER BY created_at DESC LIMIT $`+fmt.Sprintf("%d", len(args)-1)+` OFFSET $`+fmt.Sprintf("%d", len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit: %w", err)
	}
	defer rows.Close()
	list := make([]entity.Audit, 0)
	for rows.Next() {
		a := entity.Audit{}
		if err := rows.Scan(&a.ID, &a.AdminID, &a.Username, &a.Action, &a.TargetType, &a.TargetID, &a.Extra, &a.ClientIP, &a.CreatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, a)
	}
	return list, total, rows.Err()
}
