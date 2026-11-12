package service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/web3chat/txchat/dtalk/service/chatclean/config"
	"github.com/web3chat/txchat/dtalk/service/chatclean/dao"
)

const runIntegrationEnv = "DTALK_CHATCLEAN_RUN_INTEGRATION"

type privateChatFixture struct {
	uidA        string
	uidB        string
	uidC        string
	now         int64
	recentMIDs  []int64
	oldMIDs     []int64
	otherPairID int64
	allMIDs     []int64
}

func TestIntegrationService_CleanPrivateChat_CustomRange(t *testing.T) {
	svc, db := newIntegrationService(t)
	fixture := seedPrivateChatFixture(t, db, "clean")

	startTime := fixture.now - int64(24*time.Hour/time.Millisecond)
	endTime := fixture.now

	deleted, err := svc.CleanPrivateChat(context.Background(), fixture.uidA, fixture.uidB, DateOptionCustom, startTime, endTime)
	if err != nil {
		t.Fatalf("CleanPrivateChat returned error: %v", err)
	}
	if deleted != int64(len(fixture.recentMIDs)) {
		t.Fatalf("deleted count mismatch: got %d, want %d", deleted, len(fixture.recentMIDs))
	}

	for _, mid := range fixture.recentMIDs {
		assertMessageDeleted(t, db, mid)
	}
	for _, mid := range fixture.oldMIDs {
		assertMessageExists(t, db, mid)
	}
	assertMessageExists(t, db, fixture.otherPairID)

	auditCount := countRows(t, db, `
		SELECT COUNT(*) FROM dtalk_clean_audit
		WHERE operator_id = ? AND target_id = ? AND source = 0 AND deleted_cnt = ?`,
		fixture.uidA, fixture.uidB, deleted)
	if auditCount != 1 {
		t.Fatalf("audit row mismatch: got %d, want 1", auditCount)
	}
}

func TestIntegrationService_SetAndGetAutoDeleteConfig_RoundTrip(t *testing.T) {
	svc, db := newIntegrationService(t)
	fixture := seedPrivateChatFixture(t, db, "config")
	uidA, uidB := normalizeUidPair(fixture.uidA, fixture.uidB)

	cfg, err := svc.SetAutoDeleteConfig(context.Background(), fixture.uidB, fixture.uidA, AutoDelete7Days)
	if err != nil {
		t.Fatalf("SetAutoDeleteConfig returned error: %v", err)
	}
	if cfg.Option != int8(AutoDelete7Days) {
		t.Fatalf("set option mismatch: got %d, want %d", cfg.Option, AutoDelete7Days)
	}
	if cfg.UpdatedBy != fixture.uidB {
		t.Fatalf("updatedBy mismatch: got %q, want %q", cfg.UpdatedBy, fixture.uidB)
	}

	got, err := svc.GetAutoDeleteConfig(context.Background(), fixture.uidA, fixture.uidB)
	if err != nil {
		t.Fatalf("GetAutoDeleteConfig returned error: %v", err)
	}
	if got.Option != int8(AutoDelete7Days) {
		t.Fatalf("get option mismatch: got %d, want %d", got.Option, AutoDelete7Days)
	}
	if got.UpdatedBy != fixture.uidB {
		t.Fatalf("get updatedBy mismatch: got %q, want %q", got.UpdatedBy, fixture.uidB)
	}

	rowCount := countRows(t, db, `
		SELECT COUNT(*) FROM dtalk_auto_delete_config
		WHERE uid_a = ? AND uid_b = ? AND option = ? AND updated_by = ?`,
		uidA, uidB, int8(AutoDelete7Days), fixture.uidB)
	if rowCount != 1 {
		t.Fatalf("auto delete config row mismatch: got %d, want 1", rowCount)
	}
}

func TestIntegrationService_RunAutoDeleteOnce_DeletesExpiredRowsOnly(t *testing.T) {
	svc, db := newIntegrationService(t)
	fixture := seedPrivateChatFixture(t, db, "auto")
	uidA, uidB := normalizeUidPair(fixture.uidA, fixture.uidB)

	cfg := &dao.AutoDeleteConfig{
		UidA:      uidA,
		UidB:      uidB,
		Option:    int8(AutoDelete7Days),
		UpdatedBy: fixture.uidA,
		UpdatedAt: fixture.now,
	}

	deleted, err := svc.RunAutoDeleteOnce(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunAutoDeleteOnce returned error: %v", err)
	}
	if deleted != int64(len(fixture.oldMIDs)) {
		t.Fatalf("deleted count mismatch: got %d, want %d", deleted, len(fixture.oldMIDs))
	}

	for _, mid := range fixture.oldMIDs {
		assertMessageDeleted(t, db, mid)
	}
	for _, mid := range fixture.recentMIDs {
		assertMessageExists(t, db, mid)
	}
	assertMessageExists(t, db, fixture.otherPairID)

	auditCount := countRows(t, db, `
		SELECT COUNT(*) FROM dtalk_clean_audit
		WHERE operator_id = 'scheduler' AND target_id = ? AND source = 1 AND deleted_cnt = ?`,
		uidB, deleted)
	if auditCount != 1 {
		t.Fatalf("scheduler audit row mismatch: got %d, want 1", auditCount)
	}
}

func newIntegrationService(t *testing.T) (*Service, *sql.DB) {
	t.Helper()
	requireIntegrationEnabled(t)

	cfg := config.Default()
	cfg.MySQL.Host = envOrDefault("DTALK_CHATCLEAN_TEST_MYSQL_HOST", cfg.MySQL.Host)
	cfg.MySQL.Port = int32(envIntOrDefault("DTALK_CHATCLEAN_TEST_MYSQL_PORT", int(cfg.MySQL.Port)))
	cfg.MySQL.User = envOrDefault("DTALK_CHATCLEAN_TEST_MYSQL_USER", cfg.MySQL.User)
	cfg.MySQL.Pwd = envOrDefault("DTALK_CHATCLEAN_TEST_MYSQL_PWD", cfg.MySQL.Pwd)
	cfg.MySQL.Db = envOrDefault("DTALK_CHATCLEAN_TEST_MYSQL_DB", cfg.MySQL.Db)

	svc := New(cfg)
	t.Cleanup(func() {
		svc.Shutdown(context.Background())
	})

	db := svc.Dao().DB()
	ensureTestSchema(t, db)
	return svc, db
}

func requireIntegrationEnabled(t *testing.T) {
	t.Helper()
	if os.Getenv(runIntegrationEnv) != "1" {
		t.Skipf("跳过 chatclean 集成测试；如需执行，请设置 %s=1 并配置测试库环境变量", runIntegrationEnv)
	}
}

func ensureTestSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS dtalk_msg_content (
			mid BIGINT UNSIGNED NOT NULL,
			seq VARCHAR(40) NOT NULL,
			sender_id VARCHAR(40) NOT NULL,
			receiver_id VARCHAR(40) NOT NULL,
			msg_type TINYINT UNSIGNED NOT NULL,
			content LONGTEXT NOT NULL,
			create_time BIGINT NOT NULL,
			source VARCHAR(1024) DEFAULT NULL,
			reference VARCHAR(255) DEFAULT NULL,
			PRIMARY KEY (mid),
			KEY idx_sender_id_seq (sender_id, seq)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS dtalk_msg_relation (
			mid BIGINT UNSIGNED NOT NULL,
			owner_uid VARCHAR(40) NOT NULL,
			other_uid VARCHAR(40) NOT NULL,
			type TINYINT UNSIGNED NOT NULL,
			state TINYINT UNSIGNED NOT NULL DEFAULT 0,
			create_time BIGINT NOT NULL,
			PRIMARY KEY (mid, owner_uid),
			KEY idx_owneruid_otheruid_msgid (owner_uid, other_uid, mid),
			KEY idx_owneruid_type_state (owner_uid, type, state)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS dtalk_auto_delete_config (
			id BIGINT NOT NULL AUTO_INCREMENT,
			uid_a VARCHAR(128) NOT NULL,
			uid_b VARCHAR(128) NOT NULL,
			option TINYINT NOT NULL DEFAULT 0,
			updated_by VARCHAR(128) NOT NULL DEFAULT '',
			updated_at BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (id),
			UNIQUE KEY uk_pair (uid_a, uid_b),
			KEY idx_option (option)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS dtalk_clean_audit (
			id BIGINT NOT NULL AUTO_INCREMENT,
			operator_id VARCHAR(128) NOT NULL,
			target_id VARCHAR(128) NOT NULL,
			source TINYINT NOT NULL DEFAULT 0,
			date_option TINYINT NOT NULL DEFAULT 0,
			start_time BIGINT NOT NULL DEFAULT 0,
			end_time BIGINT NOT NULL DEFAULT 0,
			deleted_cnt BIGINT NOT NULL DEFAULT 0,
			created_at BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (id),
			KEY idx_operator (operator_id),
			KEY idx_created (created_at),
			KEY idx_source (source)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("ensure schema failed: %v", err)
		}
	}
}

func seedPrivateChatFixture(t *testing.T, db *sql.DB, label string) *privateChatFixture {
	t.Helper()

	suffix := time.Now().UnixNano()
	uidA := fmt.Sprintf("a%d", suffix%1000000000)
	uidB := fmt.Sprintf("b%d", suffix%1000000000)
	uidC := fmt.Sprintf("c%d", suffix%1000000000)
	now := time.Now().UnixMilli()
	baseMid := time.Now().UnixNano() / 1000

	fixture := &privateChatFixture{
		uidA: uidA,
		uidB: uidB,
		uidC: uidC,
		now:  now,
	}

	// 近 24 小时内的 A/B 会话，两条，应该在手动清理测试中被删除。
	fixture.recentMIDs = []int64{baseMid + 1, baseMid + 2}
	insertPrivateMessage(t, db, fixture.recentMIDs[0], uidA, uidB, now-int64(2*time.Hour/time.Millisecond), fmt.Sprintf("%s_recent_1", label))
	insertPrivateMessage(t, db, fixture.recentMIDs[1], uidB, uidA, now-int64(3*time.Hour/time.Millisecond), fmt.Sprintf("%s_recent_2", label))

	// 超过 7 天的 A/B 会话，供自动删除测试命中。
	fixture.oldMIDs = []int64{baseMid + 3}
	insertPrivateMessage(t, db, fixture.oldMIDs[0], uidA, uidB, now-int64(10*24*time.Hour/time.Millisecond), fmt.Sprintf("%s_old_1", label))

	// 其他会话，不应被删。
	fixture.otherPairID = baseMid + 4
	insertPrivateMessage(t, db, fixture.otherPairID, uidA, uidC, now-int64(2*time.Hour/time.Millisecond), fmt.Sprintf("%s_other_1", label))

	fixture.allMIDs = append(fixture.allMIDs, fixture.recentMIDs...)
	fixture.allMIDs = append(fixture.allMIDs, fixture.oldMIDs...)
	fixture.allMIDs = append(fixture.allMIDs, fixture.otherPairID)

	t.Cleanup(func() {
		cleanupFixture(t, db, fixture)
	})

	return fixture
}

func insertPrivateMessage(t *testing.T, db *sql.DB, mid int64, senderID, receiverID string, createTime int64, seq string) {
	t.Helper()

	if _, err := db.Exec(`
		INSERT INTO dtalk_msg_content(mid, seq, sender_id, receiver_id, msg_type, content, create_time, source, reference)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		mid, seq, senderID, receiverID, 1, "{}", createTime, "", "",
	); err != nil {
		t.Fatalf("insert dtalk_msg_content failed: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO dtalk_msg_relation(mid, owner_uid, other_uid, type, state, create_time)
		VALUES (?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?)`,
		mid, senderID, receiverID, 0, 1, createTime,
		mid, receiverID, senderID, 1, 1, createTime,
	); err != nil {
		t.Fatalf("insert dtalk_msg_relation failed: %v", err)
	}
}

func cleanupFixture(t *testing.T, db *sql.DB, fixture *privateChatFixture) {
	t.Helper()

	if len(fixture.allMIDs) > 0 {
		args := make([]interface{}, 0, len(fixture.allMIDs))
		placeholders := make([]byte, 0, len(fixture.allMIDs)*2-1)
		for i, mid := range fixture.allMIDs {
			if i > 0 {
				placeholders = append(placeholders, ',')
			}
			placeholders = append(placeholders, '?')
			args = append(args, mid)
		}

		if _, err := db.Exec("DELETE FROM dtalk_msg_relation WHERE mid IN ("+string(placeholders)+")", args...); err != nil {
			t.Fatalf("cleanup dtalk_msg_relation failed: %v", err)
		}
		if _, err := db.Exec("DELETE FROM dtalk_msg_content WHERE mid IN ("+string(placeholders)+")", args...); err != nil {
			t.Fatalf("cleanup dtalk_msg_content failed: %v", err)
		}
	}

	if _, err := db.Exec(`
		DELETE FROM dtalk_auto_delete_config
		WHERE (uid_a = ? OR uid_b = ?) OR (uid_a = ? OR uid_b = ?) OR (uid_a = ? OR uid_b = ?)`,
		fixture.uidA, fixture.uidA,
		fixture.uidB, fixture.uidB,
		fixture.uidC, fixture.uidC,
	); err != nil {
		t.Fatalf("cleanup dtalk_auto_delete_config failed: %v", err)
	}

	if _, err := db.Exec(`
		DELETE FROM dtalk_clean_audit
		WHERE operator_id IN (?, ?, 'scheduler') OR target_id IN (?, ?, ?)`,
		fixture.uidA, fixture.uidB,
		fixture.uidA, fixture.uidB, fixture.uidC,
	); err != nil {
		t.Fatalf("cleanup dtalk_clean_audit failed: %v", err)
	}
}

func assertMessageExists(t *testing.T, db *sql.DB, mid int64) {
	t.Helper()
	if got := countRows(t, db, "SELECT COUNT(*) FROM dtalk_msg_content WHERE mid = ?", mid); got != 1 {
		t.Fatalf("message %d should exist in dtalk_msg_content, got count=%d", mid, got)
	}
	if got := countRows(t, db, "SELECT COUNT(*) FROM dtalk_msg_relation WHERE mid = ?", mid); got == 0 {
		t.Fatalf("message %d should exist in dtalk_msg_relation, got count=0", mid)
	}
}

func assertMessageDeleted(t *testing.T, db *sql.DB, mid int64) {
	t.Helper()
	if got := countRows(t, db, "SELECT COUNT(*) FROM dtalk_msg_content WHERE mid = ?", mid); got != 0 {
		t.Fatalf("message %d should be deleted from dtalk_msg_content, got count=%d", mid, got)
	}
	if got := countRows(t, db, "SELECT COUNT(*) FROM dtalk_msg_relation WHERE mid = ?", mid); got != 0 {
		t.Fatalf("message %d should be deleted from dtalk_msg_relation, got count=%d", mid, got)
	}
}

func countRows(t *testing.T, db *sql.DB, query string, args ...interface{}) int64 {
	t.Helper()
	var count int64
	if err := db.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatalf("countRows failed: %v", err)
	}
	return count
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
