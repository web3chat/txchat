package dao

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/web3chat/txchat/dtalk/pkg/mysql"
	"github.com/web3chat/txchat/dtalk/service/chatadmin/model/biz"
)

const (
	maxAuditFilters        = 20
	_CreateAdminAuditTable = `CREATE TABLE IF NOT EXISTS dtalk_admin_audit (
  id          BIGINT       NOT NULL AUTO_INCREMENT,
  operator    VARCHAR(128) NOT NULL DEFAULT '' COMMENT '操作人地址',
  action      VARCHAR(64)  NOT NULL COMMENT '动作',
  target_type VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '对象类型',
  target_id   VARCHAR(128) NOT NULL DEFAULT '' COMMENT '对象 ID',
  extra       VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '补充说明',
  client_ip   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '来源 IP',
  created_at  BIGINT       NOT NULL COMMENT '操作时间，Unix 毫秒',
  PRIMARY KEY (id),
  KEY idx_dtalk_admin_audit_created (created_at),
  KEY idx_dtalk_admin_audit_action (action),
  KEY idx_dtalk_admin_audit_target (target_type, target_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='运营台操作留痕'`
	_InsertAdminAudit = `INSERT INTO dtalk_admin_audit (operator, action, target_type, target_id, extra, client_ip, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
)

// AdminAuditDAO 操作留痕，表在 dtalk_record。
type AdminAuditDAO interface {
	EnsureTable(ctx context.Context) error
	Insert(ctx context.Context, rec *biz.AdminAudit) error
	List(ctx context.Context, actions, targetTypes []string, targetID, operator string, page, pageSize int32) ([]*biz.AdminAudit, int64, error)
}

type adminAuditDAO struct {
	mysql *mysql.MysqlConn
}

// NewAdminAuditDAO 使用 dtalk_record 连接。
func NewAdminAuditDAO(mysqlRecord *mysql.MysqlConn) AdminAuditDAO {
	return &adminAuditDAO{mysql: mysqlRecord}
}

func (d *adminAuditDAO) EnsureTable(ctx context.Context) error {
	if d == nil || d.mysql == nil {
		return fmt.Errorf("dtalk_record connection is nil")
	}
	if _, _, err := d.mysql.Exec(_CreateAdminAuditTable); err != nil {
		fmt.Printf("[AdminAuditDAO] EnsureTable failed: err=%v\n", err)
		return err
	}
	return nil
}

func (d *adminAuditDAO) Insert(ctx context.Context, rec *biz.AdminAudit) error {
	if d == nil || d.mysql == nil {
		return fmt.Errorf("dtalk_record connection is nil")
	}
	if rec == nil || strings.TrimSpace(rec.Action) == "" {
		return fmt.Errorf("audit action is required")
	}
	createdAt := rec.CreatedAt
	if createdAt <= 0 {
		createdAt = time.Now().UnixMilli()
	}
	_, _, err := d.mysql.Exec(_InsertAdminAudit,
		clipAudit(rec.Operator, 128),
		clipAudit(rec.Action, 64),
		clipAudit(rec.TargetType, 32),
		clipAudit(rec.TargetID, 128),
		clipAudit(rec.Extra, 1000),
		clipAudit(rec.ClientIP, 64),
		createdAt,
	)
	if err != nil {
		fmt.Printf("[AdminAuditDAO] Insert failed: operator=%s action=%s targetType=%s targetId=%s err=%v\n",
			rec.Operator, rec.Action, rec.TargetType, rec.TargetID, err)
		return err
	}
	return nil
}

func (d *adminAuditDAO) List(ctx context.Context, actions, targetTypes []string, targetID, operator string, page, pageSize int32) ([]*biz.AdminAudit, int64, error) {
	if d == nil || d.mysql == nil {
		return nil, 0, fmt.Errorf("dtalk_record connection is nil")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	where, args := buildAuditWhere(actions, targetTypes, targetID, operator)
	countRecords, err := d.mysql.Query("SELECT COUNT(*) AS count FROM dtalk_admin_audit"+where, args...)
	if err != nil {
		fmt.Printf("[AdminAuditDAO] List count failed: actions=%v targetTypes=%v targetId=%s operator=%s err=%v\n",
			actions, targetTypes, targetID, operator, err)
		return nil, 0, err
	}
	total := int64(0)
	if len(countRecords) > 0 {
		total, _ = strconv.ParseInt(countRecords[0]["count"], 10, 64)
	}
	offset := (page - 1) * pageSize
	listArgs := append(append([]interface{}{}, args...), pageSize, offset)
	records, err := d.mysql.Query(
		"SELECT id, operator, action, target_type, target_id, extra, client_ip, created_at FROM dtalk_admin_audit"+where+" ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?",
		listArgs...,
	)
	if err != nil {
		fmt.Printf("[AdminAuditDAO] List failed: actions=%v targetTypes=%v targetId=%s operator=%s page=%d pageSize=%d err=%v\n",
			actions, targetTypes, targetID, operator, page, pageSize, err)
		return nil, 0, err
	}
	list := make([]*biz.AdminAudit, 0, len(records))
	for _, row := range records {
		item := &biz.AdminAudit{}
		item.ID, _ = strconv.ParseInt(row["id"], 10, 64)
		item.Operator = row["operator"]
		item.Action = row["action"]
		item.TargetType = row["target_type"]
		item.TargetID = row["target_id"]
		item.Extra = row["extra"]
		item.ClientIP = row["client_ip"]
		item.CreatedAt, _ = strconv.ParseInt(row["created_at"], 10, 64)
		list = append(list, item)
	}
	return list, total, nil
}

func buildAuditWhere(actions, targetTypes []string, targetID, operator string) (string, []interface{}) {
	var conds []string
	var args []interface{}
	if actions = uniqueAuditFilters(actions); len(actions) > 0 {
		conds = append(conds, "action IN ("+placeholders(len(actions))+")")
		for _, v := range actions {
			args = append(args, v)
		}
	}
	if targetTypes = uniqueAuditFilters(targetTypes); len(targetTypes) > 0 {
		conds = append(conds, "target_type IN ("+placeholders(len(targetTypes))+")")
		for _, v := range targetTypes {
			args = append(args, v)
		}
	}
	if targetID = strings.TrimSpace(targetID); targetID != "" {
		conds = append(conds, "target_id = ?")
		args = append(args, targetID)
	}
	if operator = strings.TrimSpace(operator); operator != "" {
		conds = append(conds, "operator = ?")
		args = append(args, operator)
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func uniqueAuditFilters(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
		if len(out) >= maxAuditFilters {
			break
		}
	}
	return out
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func clipAudit(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n])
}
