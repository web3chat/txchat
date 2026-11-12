package dao

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const batchSize = 500

// CleanPrivateChat 删除两个用户在指定时间范围内的私聊记录。
// 采用分批删除策略，避免长事务锁表。
// 返回删除的 dtalk_msg_content 条数。
func (d *Dao) CleanPrivateChat(ctx context.Context, uidA, uidB string, startTime, endTime int64) (int64, error) {
	var totalDeleted int64
	for {
		n, err := d.cleanBatch(ctx, uidA, uidB, startTime, endTime)
		if err != nil {
			return totalDeleted, err
		}
		totalDeleted += n
		if n < int64(batchSize) {
			break
		}
		// 避免瞬时高压，每批次间隔一点时间
		time.Sleep(10 * time.Millisecond)
	}
	return totalDeleted, nil
}

// cleanBatch 单批次删除，使用事务保证一致性。
func (d *Dao) cleanBatch(ctx context.Context, uidA, uidB string, startTime, endTime int64) (int64, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// 1. 找出本批次需要删除的 mid 列表（通过关系表定位 uid 对）
	relationTbl := fmt.Sprintf("%s.dtalk_msg_relation", d.recordDb)
	rows, err := tx.QueryContext(ctx, fmt.Sprintf(`
		SELECT DISTINCT mid
		FROM %s
		WHERE ((owner_uid = ? AND other_uid = ?) OR (owner_uid = ? AND other_uid = ?))
		  AND create_time >= ? AND create_time <= ?
		LIMIT ?`, relationTbl),
		uidA, uidB, uidB, uidA, startTime, endTime, batchSize)
	if err != nil {
		return 0, err
	}

	var mids []interface{}
	for rows.Next() {
		var mid int64
		if err = rows.Scan(&mid); err != nil {
			rows.Close()
			return 0, err
		}
		mids = append(mids, mid)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return 0, err
	}
	if len(mids) == 0 {
		tx.Commit()
		return 0, nil
	}

	// 2. 构造 IN 占位符
	placeholder := buildPlaceholders(len(mids))

	// 3. 删除关系表（两侧）
	_, err = tx.ExecContext(ctx,
		fmt.Sprintf("DELETE FROM %s WHERE mid IN (%s)", relationTbl, placeholder), mids...)
	if err != nil {
		return 0, err
	}

	// 4. 删除内容表，只删除确实是双方私聊的消息（sender/receiver 匹配）
	contentTbl := fmt.Sprintf("%s.dtalk_msg_content", d.recordDb)
	res, err := tx.ExecContext(ctx,
		fmt.Sprintf("DELETE FROM %s WHERE mid IN (%s) AND ((sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?))", contentTbl, placeholder),
		append(mids, uidA, uidB, uidB, uidA)...)
	if err != nil {
		return 0, err
	}

	affected, _ := res.RowsAffected()

	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return affected, nil
}

// WriteAuditLog 记录审计日志。
func (d *Dao) WriteAuditLog(ctx context.Context, operatorID, targetID string, source, dateOption int8, startTime, endTime, deletedCnt, createdAt int64) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT INTO dtalk_clean_audit
		  (operator_id, target_id, source, date_option, start_time, end_time, deleted_cnt, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		operatorID, targetID, source, dateOption, startTime, endTime, deletedCnt, createdAt)
	return err
}

// buildPlaceholders 生成 n 个 "?" 组成的列表，如 "?,?,?"
func buildPlaceholders(n int) string {
	if n == 0 {
		return ""
	}
	b := make([]byte, 0, n*2-1)
	for i := 0; i < n; i++ {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, '?')
	}
	return string(b)
}

// AutoDeleteConfig 自动删除配置记录
type AutoDeleteConfig struct {
	UidA      string
	UidB      string
	Option    int8
	UpdatedBy string
	UpdatedAt int64
}

// UpsertAutoDeleteConfig 保存或更新自动删除配置（同一对话唯一）。
func (d *Dao) UpsertAutoDeleteConfig(ctx context.Context, uidA, uidB string, option int8, updatedBy string, updatedAt int64) error {
	_, err := d.db.ExecContext(ctx,
		"INSERT INTO dtalk_auto_delete_config (uid_a, uid_b, `option`, updated_by, updated_at)\n"+
			"VALUES (?, ?, ?, ?, ?)\n"+
			"ON DUPLICATE KEY UPDATE `option` = VALUES(`option`), updated_by = VALUES(updated_by), updated_at = VALUES(updated_at)",
		uidA, uidB, option, updatedBy, updatedAt)
	return err
}

// GetAutoDeleteConfig 查询自动删除配置。
func (d *Dao) GetAutoDeleteConfig(ctx context.Context, uidA, uidB string) (*AutoDeleteConfig, error) {
	row := d.db.QueryRowContext(ctx,
		"SELECT uid_a, uid_b, `option`, updated_by, updated_at\n"+
			"FROM dtalk_auto_delete_config\n"+
			"WHERE uid_a = ? AND uid_b = ?", uidA, uidB)
	cfg := &AutoDeleteConfig{}
	err := row.Scan(&cfg.UidA, &cfg.UidB, &cfg.Option, &cfg.UpdatedBy, &cfg.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return cfg, err
}

// ListNonNullAutoDeleteConfigs 查询所有启用了自动删除（option != 0）的配置，供调度器使用。
func (d *Dao) ListNonNullAutoDeleteConfigs(ctx context.Context) ([]*AutoDeleteConfig, error) {
	rows, err := d.db.QueryContext(ctx,
		"SELECT uid_a, uid_b, `option`, updated_by, updated_at\n"+
			"FROM dtalk_auto_delete_config\n"+
			"WHERE `option` != 0")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cfgs []*AutoDeleteConfig
	for rows.Next() {
		cfg := &AutoDeleteConfig{}
		if err = rows.Scan(&cfg.UidA, &cfg.UidB, &cfg.Option, &cfg.UpdatedBy, &cfg.UpdatedAt); err != nil {
			return nil, err
		}
		cfgs = append(cfgs, cfg)
	}
	return cfgs, rows.Err()
}
