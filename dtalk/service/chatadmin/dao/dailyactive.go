package dao

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/web3chat/txchat/dtalk/pkg/mysql"
	"github.com/web3chat/txchat/dtalk/service/chatadmin/model/biz"
)

const (
	_CreateDailyActiveTable = `CREATE TABLE IF NOT EXISTS chatadmin_daily_active (
        id BIGINT PRIMARY KEY AUTO_INCREMENT,
        stat_date DATE NOT NULL,
        active_users BIGINT NOT NULL DEFAULT 0,
        active_groups BIGINT NOT NULL DEFAULT 0,
        total_users BIGINT NOT NULL DEFAULT 0,
        total_groups BIGINT NOT NULL DEFAULT 0,
        created_at BIGINT NOT NULL,
        updated_at BIGINT NOT NULL,
        UNIQUE KEY uk_stat_date (stat_date),
        KEY idx_stat_date (stat_date)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

	_UpsertDailyActive = `INSERT INTO chatadmin_daily_active
        (stat_date, active_users, active_groups, total_users, total_groups, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?)
        ON DUPLICATE KEY UPDATE
        active_users = VALUES(active_users),
        active_groups = VALUES(active_groups),
        total_users = IF(? = 1, total_users, VALUES(total_users)),
        total_groups = IF(? = 1, total_groups, VALUES(total_groups)),
        updated_at = VALUES(updated_at)`

	_CountDailyActive = `SELECT COUNT(*) AS count FROM chatadmin_daily_active`
	_ListDailyActive  = `SELECT DATE_FORMAT(stat_date, '%Y-%m-%d') AS stat_date,
        active_users, active_groups, total_users, total_groups, updated_at
        FROM chatadmin_daily_active`
)

// DailyActiveDAO 每日活跃入库 DAO
type DailyActiveDAO interface {
	EnsureTable(ctx context.Context) error
	Upsert(ctx context.Context, rec *biz.DailyActiveRecord, keepCumulative bool) error
	List(ctx context.Context, startDate, endDate string, dates []string, page, pageSize int32) ([]*biz.DailyActiveRecord, int64, error)
}

type dailyActiveDAO struct {
	mysql *mysql.MysqlConn
}

// NewDailyActiveDAO 创建每日活跃 DAO（写入 dtalk 库）
func NewDailyActiveDAO(mysqlConn *mysql.MysqlConn) DailyActiveDAO {
	return &dailyActiveDAO{mysql: mysqlConn}
}

func (d *dailyActiveDAO) EnsureTable(ctx context.Context) error {
	if d.mysql == nil {
		return fmt.Errorf("mysql connection is nil")
	}
	_, _, err := d.mysql.Exec(_CreateDailyActiveTable)
	if err != nil {
		fmt.Printf("[DailyActiveDAO] EnsureTable failed: err=%v\n", err)
		return err
	}
	return nil
}

func (d *dailyActiveDAO) Upsert(ctx context.Context, rec *biz.DailyActiveRecord, keepCumulative bool) error {
	if d.mysql == nil {
		return fmt.Errorf("mysql connection is nil")
	}
	if rec == nil || rec.StatDate == "" {
		return fmt.Errorf("statDate is required")
	}
	now := time.Now().UnixMilli()
	createdAt := now
	if rec.UpdatedAt > 0 {
		createdAt = rec.UpdatedAt
	}
	keep := 0
	if keepCumulative {
		keep = 1
	}
	_, _, err := d.mysql.Exec(_UpsertDailyActive,
		rec.StatDate,
		rec.ActiveUsers,
		rec.ActiveGroups,
		rec.TotalUsers,
		rec.TotalGroups,
		createdAt,
		now,
		keep,
		keep,
	)
	if err != nil {
		fmt.Printf("[DailyActiveDAO] Upsert failed: statDate=%s activeUsers=%d activeGroups=%d err=%v\n",
			rec.StatDate, rec.ActiveUsers, rec.ActiveGroups, err)
		return err
	}
	fmt.Printf("[DailyActiveDAO] Upsert ok: statDate=%s activeUsers=%d activeGroups=%d totalUsers=%d totalGroups=%d\n",
		rec.StatDate, rec.ActiveUsers, rec.ActiveGroups, rec.TotalUsers, rec.TotalGroups)
	return nil
}

func (d *dailyActiveDAO) List(ctx context.Context, startDate, endDate string, dates []string, page, pageSize int32) ([]*biz.DailyActiveRecord, int64, error) {
	if d.mysql == nil {
		return nil, 0, fmt.Errorf("mysql connection is nil")
	}
	where, args := buildDailyActiveWhere(startDate, endDate, dates)
	countSQL := _CountDailyActive + where
	countRecords, err := d.mysql.Query(countSQL, args...)
	if err != nil {
		fmt.Printf("[DailyActiveDAO] List count failed: startDate=%s endDate=%s dates=%v err=%v\n",
			startDate, endDate, dates, err)
		return nil, 0, err
	}
	var total int64
	if len(countRecords) > 0 {
		if v, ok := countRecords[0]["count"]; ok {
			total, _ = strconv.ParseInt(v, 10, 64)
		}
	}

	offset := (page - 1) * pageSize
	listSQL := _ListDailyActive + where + " ORDER BY stat_date DESC LIMIT ?, ?"
	listArgs := append(append([]interface{}{}, args...), offset, pageSize)
	records, err := d.mysql.Query(listSQL, listArgs...)
	if err != nil {
		fmt.Printf("[DailyActiveDAO] List query failed: startDate=%s endDate=%s page=%d pageSize=%d err=%v\n",
			startDate, endDate, page, pageSize, err)
		return nil, 0, err
	}

	list := make([]*biz.DailyActiveRecord, 0, len(records))
	for _, r := range records {
		item := &biz.DailyActiveRecord{}
		if v, ok := r["stat_date"]; ok {
			item.StatDate = v
		}
		if v, ok := r["active_users"]; ok {
			item.ActiveUsers, _ = strconv.ParseInt(v, 10, 64)
		}
		if v, ok := r["active_groups"]; ok {
			item.ActiveGroups, _ = strconv.ParseInt(v, 10, 64)
		}
		if v, ok := r["total_users"]; ok {
			item.TotalUsers, _ = strconv.ParseInt(v, 10, 64)
		}
		if v, ok := r["total_groups"]; ok {
			item.TotalGroups, _ = strconv.ParseInt(v, 10, 64)
		}
		if v, ok := r["updated_at"]; ok {
			item.UpdatedAt, _ = strconv.ParseInt(v, 10, 64)
		}
		list = append(list, item)
	}
	return list, total, nil
}

func buildDailyActiveWhere(startDate, endDate string, dates []string) (string, []interface{}) {
	conds := make([]string, 0, 3)
	args := make([]interface{}, 0, 8)
	cleanDates := make([]string, 0, len(dates))
	for _, d := range dates {
		d = strings.TrimSpace(d)
		if d != "" {
			cleanDates = append(cleanDates, d)
		}
	}
	if len(cleanDates) > 0 {
		placeholders := make([]string, len(cleanDates))
		for i, d := range cleanDates {
			placeholders[i] = "?"
			args = append(args, d)
		}
		conds = append(conds, "stat_date IN ("+strings.Join(placeholders, ",")+")")
	}
	if startDate != "" {
		conds = append(conds, "stat_date >= ?")
		args = append(args, startDate)
	}
	if endDate != "" {
		conds = append(conds, "stat_date <= ?")
		args = append(args, endDate)
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}
