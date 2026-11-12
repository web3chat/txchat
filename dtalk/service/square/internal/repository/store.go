package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) EnsureSchema(ctx context.Context, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read schema %s: %w", path, err)
	}
	// pgx 对无占位符的整文件 Exec 常只跑第一条或报 prepared 多语句错误。
	// 已有库 CREATE TABLE IF NOT EXISTS 直接成功，后面的 ALTER 就不会执行。
	for i, stmt := range splitSQLStatements(string(raw)) {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			slog.Error("ensure schema statement failed", "path", path, "index", i, "sql", truncateSQL(stmt), "err", err)
			return fmt.Errorf("exec schema stmt %d: %w", i+1, err)
		}
	}
	if err := s.migrate(ctx); err != nil {
		return err
	}
	slog.Info("schema ensured", "path", path)
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`ALTER TABLE square_post ADD COLUMN IF NOT EXISTS title VARCHAR(64) NOT NULL DEFAULT ''`,
		`ALTER TABLE square_post ADD COLUMN IF NOT EXISTS intro TEXT NOT NULL DEFAULT ''`,
		`CREATE TABLE IF NOT EXISTS square_follow (
    follower_addr VARCHAR(64) NOT NULL,
    target_addr   VARCHAR(64) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (follower_addr, target_addr),
    CHECK (follower_addr <> target_addr)
)`,
		`CREATE INDEX IF NOT EXISTS idx_square_follow_target ON square_follow (target_addr, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS square_report (
    id            BIGSERIAL PRIMARY KEY,
    reporter_addr VARCHAR(64) NOT NULL,
    target_type   VARCHAR(16) NOT NULL,
    target_id     BIGINT NOT NULL,
    reason        VARCHAR(32) NOT NULL,
    detail        TEXT NOT NULL DEFAULT '',
    status        SMALLINT NOT NULL DEFAULT 0,
    handled_by    VARCHAR(64) NOT NULL DEFAULT '',
    handled_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (reporter_addr, target_type, target_id)
)`,
		`CREATE INDEX IF NOT EXISTS idx_square_report_status ON square_report (status, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_square_report_target ON square_report (target_type, target_id)`,
		`ALTER TABLE square_post ADD COLUMN IF NOT EXISTS pinned_at TIMESTAMPTZ`,
		`CREATE INDEX IF NOT EXISTS idx_square_post_pinned ON square_post (pinned_at DESC) WHERE pinned_at IS NOT NULL`,
		`ALTER TABLE square_post ADD COLUMN IF NOT EXISTS tag VARCHAR(32) NOT NULL DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS idx_square_post_tag ON square_post (tag) WHERE tag <> ''`,
		`ALTER TABLE square_content_moderation ADD COLUMN IF NOT EXISTS overridden_by VARCHAR(64)`,
		`ALTER TABLE square_content_moderation ADD COLUMN IF NOT EXISTS overridden_at TIMESTAMPTZ`,
		`ALTER TABLE square_content_moderation ADD COLUMN IF NOT EXISTS final_verdict VARCHAR(16)`,
		`CREATE TABLE IF NOT EXISTS square_setting (
    key        VARCHAR(64) PRIMARY KEY,
    value      TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`,
		`INSERT INTO square_setting (key, value) VALUES
    ('gate_mode', 'open'),
    ('gate_message', '广场功能已经关闭'),
    ('allow_officials', '1'),
    ('moderation_level', 'default')
ON CONFLICT (key) DO NOTHING`,
		`CREATE TABLE IF NOT EXISTS square_poster_allow (
    address    VARCHAR(64) PRIMARY KEY,
    remark     TEXT NOT NULL DEFAULT '',
    operator   VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`,
		`ALTER TABLE square_official_account ADD COLUMN IF NOT EXISTS gold BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE square_official_account ADD COLUMN IF NOT EXISTS gold_at TIMESTAMPTZ`,
		`CREATE INDEX IF NOT EXISTS idx_square_official_gold ON square_official_account (gold) WHERE gold`,
		`CREATE INDEX IF NOT EXISTS idx_square_official_account_name ON square_official_account (name)`,
		`CREATE INDEX IF NOT EXISTS idx_square_official_apply_name ON square_official_apply (name) WHERE status IN ('pending', 'approved')`,
		`ALTER TABLE square_admin ADD COLUMN IF NOT EXISTS permissions TEXT`,
		`UPDATE square_post SET tag = '消息' WHERE tag = '新闻'`,
		`ALTER TABLE square_comment ADD COLUMN IF NOT EXISTS reply_to_id BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE square_comment ADD COLUMN IF NOT EXISTS reply_to_addr VARCHAR(64) NOT NULL DEFAULT ''`,
		`ALTER TABLE square_comment ADD COLUMN IF NOT EXISTS reply_to_name VARCHAR(128) NOT NULL DEFAULT ''`,
		`ALTER TABLE square_comment ADD COLUMN IF NOT EXISTS reply_count BIGINT NOT NULL DEFAULT 0`,
		`CREATE INDEX IF NOT EXISTS idx_square_comment_root ON square_comment (post_id, status, created_at) WHERE parent_id = 0`,
		`CREATE INDEX IF NOT EXISTS idx_square_comment_replies ON square_comment (parent_id, status, created_at) WHERE parent_id > 0`,
		`CREATE TABLE IF NOT EXISTS square_reply_notice (
    id           BIGSERIAL PRIMARY KEY,
    recipient    VARCHAR(64) NOT NULL,
    actor_addr   VARCHAR(64) NOT NULL,
    actor_name   VARCHAR(128) NOT NULL DEFAULT '',
    post_id      BIGINT NOT NULL,
    post_title   TEXT NOT NULL DEFAULT '',
    root_id      BIGINT NOT NULL,
    comment_id   BIGINT NOT NULL,
    reply_to_id  BIGINT NOT NULL DEFAULT 0,
    content      TEXT NOT NULL DEFAULT '',
    read_at      TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (comment_id, recipient)
)`,
		`CREATE INDEX IF NOT EXISTS idx_square_reply_notice_inbox ON square_reply_notice (recipient, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_square_reply_notice_unread ON square_reply_notice (recipient, created_at DESC) WHERE read_at IS NULL`,
		`ALTER TABLE square_post ADD COLUMN IF NOT EXISTS publish_at TIMESTAMPTZ`,
		`ALTER TABLE square_post ADD COLUMN IF NOT EXISTS expire_at TIMESTAMPTZ`,
		`ALTER TABLE square_post ADD COLUMN IF NOT EXISTS expire_after_sec BIGINT NOT NULL DEFAULT 0`,
		`CREATE INDEX IF NOT EXISTS idx_square_post_publish_at ON square_post (publish_at) WHERE publish_at IS NOT NULL AND status = 6`,
		`CREATE INDEX IF NOT EXISTS idx_square_post_expire_at ON square_post (expire_at) WHERE expire_at IS NOT NULL AND status IN (1, 2)`,
		`CREATE TABLE IF NOT EXISTS square_official_name (
    id         BIGSERIAL PRIMARY KEY,
    name       VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uniq_square_official_name ON square_official_name (name)`,
		`ALTER TABLE square_official_name ADD COLUMN IF NOT EXISTS avatar TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE square_official_name ADD COLUMN IF NOT EXISTS intro TEXT NOT NULL DEFAULT ''`,
		`CREATE TABLE IF NOT EXISTS square_admin_session (
    id         VARCHAR(64) PRIMARY KEY,
    admin_id   BIGINT NOT NULL,
    username   VARCHAR(64) NOT NULL,
    client_ip  VARCHAR(64) NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    last_seen  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`,
		`CREATE INDEX IF NOT EXISTS idx_square_admin_session_active ON square_admin_session (username, client_ip) WHERE revoked_at IS NULL`,
	}
	for _, q := range stmts {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			slog.Error("schema migrate failed", "sql", q, "err", err)
			return fmt.Errorf("schema migrate: %w", err)
		}
	}
	return nil
}

func splitSQLStatements(raw string) []string {
	var out []string
	var b strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "--") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
		if strings.HasSuffix(trim, ";") {
			stmt := strings.TrimSpace(b.String())
			b.Reset()
			stmt = strings.TrimSuffix(stmt, ";")
			stmt = strings.TrimSpace(stmt)
			if stmt != "" {
				out = append(out, stmt)
			}
		}
	}
	if rest := strings.TrimSpace(b.String()); rest != "" {
		out = append(out, rest)
	}
	return out
}

func truncateSQL(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 160 {
		return s[:160] + "…"
	}
	return s
}
