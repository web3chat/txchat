package dao

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/gomodule/redigo/redis"
)

const (
	PrivateContentTable   = "dtalk_msg_content"
	PrivateRelationTable  = "dtalk_msg_relation"
	GroupContentTable     = "dtalk_group_msg_content"
	GroupRelationTable    = "dtalk_group_msg_relation"
	redisRecordKeyPattern = "record:*"
	redisScanCount        = 100
)

func validIdent(name, fallback string) (string, error) {
	if name == "" {
		name = fallback
	}
	if name == "" {
		return "", fmt.Errorf("empty identifier")
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return "", fmt.Errorf("invalid identifier %q", name)
	}
	return name, nil
}

func checkTable(name string) error {
	switch name {
	case PrivateContentTable, PrivateRelationTable, GroupContentTable, GroupRelationTable:
		return nil
	default:
		return fmt.Errorf("retention: unexpected table %q", name)
	}
}

// DeleteExpiredContent 选出 mid、create_time 都早于截止线的正文，先删 relation 再删 content。
// 返回本批选出的 mid 数。0 表示这一侧已经没有过期正文。
func (d *Dao) DeleteExpiredContent(ctx context.Context, contentTable, relationTable string, cutoffMid, cutoffMs int64, limit int) (int, error) {
	if err := checkTable(contentTable); err != nil {
		return 0, err
	}
	if err := checkTable(relationTable); err != nil {
		return 0, err
	}
	if limit <= 0 {
		return 0, fmt.Errorf("retention: limit=%d table=%s", limit, contentTable)
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("retention begin table=%s cutoff_ms=%d: %w", contentTable, cutoffMs, err)
	}
	defer func() { _ = tx.Rollback() }()

	mids, err := selectExpiredMids(ctx, tx, d.recordDb, contentTable, cutoffMid, cutoffMs, limit)
	if err != nil {
		return 0, err
	}
	if len(mids) == 0 {
		if err = tx.Commit(); err != nil {
			return 0, fmt.Errorf("retention commit empty table=%s cutoff_ms=%d: %w", contentTable, cutoffMs, err)
		}
		return 0, nil
	}
	if err = deleteByMids(ctx, tx, d.recordDb, relationTable, mids, false, 0); err != nil {
		return 0, fmt.Errorf("retention delete relation table=%s mids=%d cutoff_ms=%d: %w", relationTable, len(mids), cutoffMs, err)
	}
	if err = deleteByMids(ctx, tx, d.recordDb, contentTable, mids, true, cutoffMs); err != nil {
		return 0, fmt.Errorf("retention delete content table=%s mids=%d cutoff_ms=%d: %w", contentTable, len(mids), cutoffMs, err)
	}
	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("retention commit table=%s mids=%d cutoff_ms=%d: %w", contentTable, len(mids), cutoffMs, err)
	}
	return len(mids), nil
}

// DeleteExpiredRelations 删除正文已经不在、但 relation 仍早于截止线的行。
// 条件与正文相同：mid 和 create_time 都早于截止线。
func (d *Dao) DeleteExpiredRelations(ctx context.Context, relationTable string, cutoffMid, cutoffMs int64, limit int) (int64, error) {
	if err := checkTable(relationTable); err != nil {
		return 0, err
	}
	if limit <= 0 {
		return 0, fmt.Errorf("retention: limit=%d table=%s", limit, relationTable)
	}
	q := fmt.Sprintf(
		"DELETE FROM %s.%s WHERE mid < ? AND create_time < ? ORDER BY mid LIMIT ?",
		d.recordDb, relationTable,
	)
	res, err := d.db.ExecContext(ctx, q, cutoffMid, cutoffMs, limit)
	if err != nil {
		return 0, fmt.Errorf("retention delete orphan table=%s cutoff_ms=%d limit=%d: %w", relationTable, cutoffMs, limit, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("retention orphan rows table=%s cutoff_ms=%d: %w", relationTable, cutoffMs, err)
	}
	return n, nil
}

func selectExpiredMids(ctx context.Context, tx *sql.Tx, recordDb, table string, cutoffMid, cutoffMs int64, limit int) ([]int64, error) {
	q := fmt.Sprintf(
		"SELECT mid FROM %s.%s WHERE mid < ? AND create_time < ? ORDER BY mid LIMIT ?",
		recordDb, table,
	)
	rows, err := tx.QueryContext(ctx, q, cutoffMid, cutoffMs, limit)
	if err != nil {
		return nil, fmt.Errorf("retention select table=%s cutoff_ms=%d limit=%d: %w", table, cutoffMs, limit, err)
	}
	defer rows.Close()

	mids := make([]int64, 0, limit)
	for rows.Next() {
		var mid int64
		if err = rows.Scan(&mid); err != nil {
			return nil, fmt.Errorf("retention scan table=%s cutoff_ms=%d: %w", table, cutoffMs, err)
		}
		mids = append(mids, mid)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("retention rows table=%s cutoff_ms=%d: %w", table, cutoffMs, err)
	}
	return mids, nil
}

func deleteByMids(ctx context.Context, tx *sql.Tx, recordDb, table string, mids []int64, restrictTime bool, cutoffMs int64) error {
	if len(mids) == 0 {
		return nil
	}
	args := make([]interface{}, 0, len(mids)+1)
	for _, mid := range mids {
		args = append(args, mid)
	}
	q := fmt.Sprintf("DELETE FROM %s.%s WHERE mid IN (%s)", recordDb, table, buildPlaceholders(len(mids)))
	if restrictTime {
		q += " AND create_time < ?"
		args = append(args, cutoffMs)
	}
	_, err := tx.ExecContext(ctx, q, args...)
	return err
}

// TrimRecordCache 按 CreateTime 删除 record:{uid} 里过期的成员。不记录成员正文。
// maxKeys 是本轮最多扫描的 key 数。
func (d *Dao) TrimRecordCache(ctx context.Context, cutoffMs uint64, maxKeys int) (removed int, more bool, err error) {
	if d.redis == nil || maxKeys <= 0 {
		return 0, false, nil
	}
	conn := d.redis.Get()
	defer conn.Close()
	if err = conn.Err(); err != nil {
		return 0, false, fmt.Errorf("retention redis conn cutoff_ms=%d: %w", cutoffMs, err)
	}

	cursor := 0
	seen := 0
	for {
		if err = ctx.Err(); err != nil {
			return removed, true, err
		}
		values, scanErr := redis.Values(conn.Do("SCAN", cursor, "MATCH", redisRecordKeyPattern, "COUNT", redisScanCount))
		if scanErr != nil {
			return removed, true, fmt.Errorf("retention redis scan cutoff_ms=%d: %w", cutoffMs, scanErr)
		}
		if len(values) != 2 {
			return removed, true, fmt.Errorf("retention redis scan reply cutoff_ms=%d", cutoffMs)
		}
		cursor, err = redis.Int(values[0], nil)
		if err != nil {
			return removed, true, fmt.Errorf("retention redis cursor cutoff_ms=%d: %w", cutoffMs, err)
		}
		keys, err := redis.Strings(values[1], nil)
		if err != nil {
			return removed, true, fmt.Errorf("retention redis keys cutoff_ms=%d: %w", cutoffMs, err)
		}
		for _, key := range keys {
			if seen >= maxKeys {
				return removed, true, nil
			}
			seen++
			n, trimErr := trimRecordKey(conn, key, cutoffMs)
			if trimErr != nil {
				return removed, true, fmt.Errorf("retention redis trim key=%s cutoff_ms=%d: %w", key, cutoffMs, trimErr)
			}
			removed += n
		}
		if cursor == 0 {
			return removed, false, nil
		}
	}
}

func trimRecordKey(conn redis.Conn, key string, cutoffMs uint64) (int, error) {
	members, err := redis.ByteSlices(conn.Do("ZRANGE", key, 0, -1))
	if err != nil {
		return 0, err
	}
	args := make([]interface{}, 0, len(members)+1)
	args = append(args, key)
	for _, member := range members {
		expired, ok := cacheMemberExpired(member, cutoffMs)
		if ok && expired {
			args = append(args, member)
		}
	}
	if len(args) == 1 {
		return 0, nil
	}
	return redis.Int(conn.Do("ZREM", args...))
}

type cacheCreateTime struct {
	CreateTime uint64
}

func cacheMemberExpired(raw []byte, cutoffMs uint64) (expired bool, ok bool) {
	var item cacheCreateTime
	if err := json.Unmarshal(raw, &item); err != nil {
		return false, false
	}
	if item.CreateTime == 0 {
		return false, true
	}
	return item.CreateTime < cutoffMs, true
}
