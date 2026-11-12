package service

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/web3chat/txchat/dtalk/service/chatclean/config"
)

const retentionTestSchema = "dtalk_record_retention_test"

func TestIntegration_GlobalRetention(t *testing.T) {
	requireIntegrationEnabled(t)

	cfg := config.Default()
	cfg.MySQL.Host = envOrDefault("DTALK_CHATCLEAN_TEST_MYSQL_HOST", cfg.MySQL.Host)
	cfg.MySQL.Port = int32(envIntOrDefault("DTALK_CHATCLEAN_TEST_MYSQL_PORT", int(cfg.MySQL.Port)))
	cfg.MySQL.User = envOrDefault("DTALK_CHATCLEAN_TEST_MYSQL_USER", cfg.MySQL.User)
	cfg.MySQL.Pwd = envOrDefault("DTALK_CHATCLEAN_TEST_MYSQL_PWD", cfg.MySQL.Pwd)
	cfg.MySQL.Db = envOrDefault("DTALK_CHATCLEAN_TEST_MYSQL_DB", cfg.MySQL.Db)
	cfg.MySQL.RecordDb = retentionTestSchema
	cfg.Retention.Enable = true
	cfg.Retention.Keep = config.Default().Retention.Keep
	cfg.Retention.MaxBatches = 1
	cfg.Scheduler.BatchSize = 1
	cfg.MinIO.Enable = false
	cfg.Redis.Enable = false

	svc := New(cfg)
	t.Cleanup(func() { svc.Shutdown(context.Background()) })
	db := svc.Dao().DB()
	if _, err := db.Exec("CREATE DATABASE IF NOT EXISTS " + retentionTestSchema); err != nil {
		t.Skipf("无法创建保留测试库: %v", err)
	}
	ensureRetentionSchema(t, db)
	wipeRetentionTables(t, db)

	now := time.Now()
	_, cutoffMid := Cutoff(now, 168*time.Hour)
	base := time.Now().UnixNano()%1_000_000 + 10
	oldTime := now.Add(-10 * 24 * time.Hour).UnixMilli()
	recentTime := now.Add(-time.Hour).UnixMilli()

	oldPrivate := int64(base + 1)
	idlePrivate := int64(base + 2)
	recentPrivate := int64(base + 3)
	futureMidPrivate := futureMid(now)
	orphanMid := int64(base + 4)
	oldGroup := int64(base + 5)
	recentGroup := int64(base + 6)
	mids := []int64{oldPrivate, idlePrivate, recentPrivate, futureMidPrivate, orphanMid, oldGroup, recentGroup}

	uidA := fmt.Sprintf("ra%d", base)
	uidB := fmt.Sprintf("rb%d", base)
	groupID := fmt.Sprintf("rg%d", base)
	insertRecord(t, db, "dtalk_msg_content", "dtalk_msg_relation", oldPrivate, uidA, uidB, oldTime)
	insertRecord(t, db, "dtalk_msg_content", "dtalk_msg_relation", idlePrivate, uidA, uidB, oldTime)
	insertRecord(t, db, "dtalk_msg_content", "dtalk_msg_relation", recentPrivate, uidA, uidB, recentTime)
	insertRecord(t, db, "dtalk_msg_content", "dtalk_msg_relation", futureMidPrivate, uidA, uidB, oldTime)
	insertRelationOnly(t, db, "dtalk_msg_relation", orphanMid, uidA, uidB, oldTime)
	insertRecord(t, db, "dtalk_group_msg_content", "dtalk_group_msg_relation", oldGroup, uidA, groupID, oldTime)
	insertRecord(t, db, "dtalk_group_msg_content", "dtalk_group_msg_relation", recentGroup, uidA, groupID, recentTime)
	t.Cleanup(func() { cleanupRetentionRows(t, db, mids) })

	if futureMidPrivate < cutoffMid {
		t.Fatalf("fixture mid %d is not above cutoff %d", futureMidPrivate, cutoffMid)
	}

	started := time.Now().UnixMilli()
	first, err := svc.RunGlobalRetention(context.Background())
	if err != nil {
		t.Fatalf("first round: %v", err)
	}
	if !first.PrivateMore {
		t.Fatalf("first round should hit the batch cap, report=%+v", first)
	}
	if countRecord(t, db, "dtalk_msg_content", recentPrivate) != 1 {
		t.Fatal("recent private message deleted")
	}
	if countRecord(t, db, "dtalk_msg_content", futureMidPrivate) != 1 {
		t.Fatal("old create_time with newer mid deleted")
	}
	if countRecord(t, db, "dtalk_msg_relation", orphanMid) != 1 {
		t.Fatal("orphan relation deleted before content backlog finished")
	}

	svc.cfg.Retention.MaxBatches = 20
	svc.cfg.Scheduler.BatchSize = 50
	second, err := svc.RunGlobalRetention(context.Background())
	if err != nil {
		t.Fatalf("second round: %v", err)
	}
	if second.PrivateMore || second.GroupMore {
		t.Fatalf("second round still capped: %+v", second)
	}
	for _, mid := range []int64{oldPrivate, idlePrivate} {
		if countRecord(t, db, "dtalk_msg_content", mid) != 0 || countRecord(t, db, "dtalk_msg_relation", mid) != 0 {
			t.Fatalf("old private mid %d still present", mid)
		}
	}
	if countRecord(t, db, "dtalk_msg_relation", orphanMid) != 0 {
		t.Fatal("orphan relation still present")
	}
	if countRecord(t, db, "dtalk_msg_content", recentPrivate) != 1 || countRecord(t, db, "dtalk_msg_relation", recentPrivate) != 2 {
		t.Fatal("recent private message missing")
	}
	if countRecord(t, db, "dtalk_group_msg_content", oldGroup) != 0 || countRecord(t, db, "dtalk_group_msg_relation", oldGroup) != 0 {
		t.Fatal("old group message still present")
	}
	if countRecord(t, db, "dtalk_group_msg_content", recentGroup) != 1 {
		t.Fatal("recent group message missing")
	}

	audit := countRows(t, db, `
		SELECT COUNT(*) FROM dtalk_clean_audit
		WHERE operator_id = 'retention' AND target_id = 'global' AND source = 2 AND created_at >= ?`, started)
	if audit < 1 {
		t.Fatalf("retention audit rows = %d", audit)
	}
	if _, err := db.Exec(`DELETE FROM dtalk_clean_audit WHERE operator_id = 'retention' AND target_id = 'global' AND created_at >= ?`, started); err != nil {
		t.Fatalf("cleanup audit: %v", err)
	}
}

func ensureRetentionSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS ` + retentionTestSchema + `.dtalk_msg_content (
			mid BIGINT UNSIGNED NOT NULL,
			seq VARCHAR(40) NOT NULL,
			sender_id VARCHAR(40) NOT NULL,
			receiver_id VARCHAR(40) NOT NULL,
			msg_type TINYINT UNSIGNED NOT NULL,
			content LONGTEXT NOT NULL,
			create_time BIGINT NOT NULL,
			source VARCHAR(1024) DEFAULT NULL,
			reference VARCHAR(255) DEFAULT NULL,
			PRIMARY KEY (mid)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS ` + retentionTestSchema + `.dtalk_msg_relation (
			mid BIGINT UNSIGNED NOT NULL,
			owner_uid VARCHAR(40) NOT NULL,
			other_uid VARCHAR(40) NOT NULL,
			type TINYINT UNSIGNED NOT NULL,
			state TINYINT UNSIGNED NOT NULL DEFAULT 0,
			create_time BIGINT NOT NULL,
			PRIMARY KEY (mid, owner_uid)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS ` + retentionTestSchema + `.dtalk_group_msg_content (
			mid BIGINT UNSIGNED NOT NULL,
			seq VARCHAR(40) NOT NULL,
			sender_id VARCHAR(40) NOT NULL,
			receiver_id VARCHAR(40) NOT NULL,
			msg_type TINYINT UNSIGNED NOT NULL,
			content LONGTEXT NOT NULL,
			create_time BIGINT NOT NULL,
			source VARCHAR(1024) DEFAULT NULL,
			reference VARCHAR(255) DEFAULT NULL,
			PRIMARY KEY (mid)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS ` + retentionTestSchema + `.dtalk_group_msg_relation (
			mid BIGINT UNSIGNED NOT NULL,
			owner_uid VARCHAR(40) NOT NULL,
			other_uid VARCHAR(40) NOT NULL,
			type TINYINT UNSIGNED NOT NULL,
			state TINYINT UNSIGNED NOT NULL DEFAULT 0,
			create_time BIGINT NOT NULL,
			PRIMARY KEY (mid, owner_uid)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("ensure retention schema: %v", err)
		}
	}
	ensureTestSchema(t, db)
}

func wipeRetentionTables(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, table := range []string{"dtalk_msg_relation", "dtalk_msg_content", "dtalk_group_msg_relation", "dtalk_group_msg_content"} {
		if _, err := db.Exec("DELETE FROM " + retentionTestSchema + "." + table); err != nil {
			t.Fatalf("wipe %s: %v", table, err)
		}
	}
}

func insertRecord(t *testing.T, db *sql.DB, contentTable, relationTable string, mid int64, sender, receiver string, createTime int64) {
	t.Helper()
	q := fmt.Sprintf(`INSERT INTO %s.%s (mid, seq, sender_id, receiver_id, msg_type, content, create_time, source, reference) VALUES (?,?,?,?,?,?,?,?,?)`, retentionTestSchema, contentTable)
	if _, err := db.Exec(q, mid, fmt.Sprintf("s%d", mid), sender, receiver, 1, "{}", createTime, "", ""); err != nil {
		t.Fatalf("insert %s mid=%d: %v", contentTable, mid, err)
	}
	insertRelationOnly(t, db, relationTable, mid, sender, receiver, createTime)
}

func insertRelationOnly(t *testing.T, db *sql.DB, relationTable string, mid int64, sender, receiver string, createTime int64) {
	t.Helper()
	q := fmt.Sprintf(`INSERT INTO %s.%s (mid, owner_uid, other_uid, type, state, create_time) VALUES (?,?,?,?,?,?),(?,?,?,?,?,?)`, retentionTestSchema, relationTable)
	if _, err := db.Exec(q,
		mid, sender, receiver, 0, 1, createTime,
		mid, receiver, sender, 1, 1, createTime,
	); err != nil {
		t.Fatalf("insert %s mid=%d: %v", relationTable, mid, err)
	}
}

func countRecord(t *testing.T, db *sql.DB, table string, mid int64) int {
	t.Helper()
	var n int
	q := fmt.Sprintf("SELECT COUNT(*) FROM %s.%s WHERE mid = ?", retentionTestSchema, table)
	if err := db.QueryRow(q, mid).Scan(&n); err != nil {
		t.Fatalf("count %s mid=%d: %v", table, mid, err)
	}
	return n
}

func cleanupRetentionRows(t *testing.T, db *sql.DB, mids []int64) {
	t.Helper()
	if len(mids) == 0 {
		return
	}
	args := make([]interface{}, len(mids))
	holders := ""
	for i, mid := range mids {
		if i > 0 {
			holders += ","
		}
		holders += "?"
		args[i] = mid
	}
	for _, table := range []string{"dtalk_msg_relation", "dtalk_msg_content", "dtalk_group_msg_relation", "dtalk_group_msg_content"} {
		q := fmt.Sprintf("DELETE FROM %s.%s WHERE mid IN (%s)", retentionTestSchema, table, holders)
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("cleanup %s: %v", table, err)
		}
	}
}

func futureMid(now time.Time) int64 {
	ms := now.Add(24 * time.Hour).UnixMilli()
	return (ms - snowflakeEpoch) << snowflakeTimeShift
}
