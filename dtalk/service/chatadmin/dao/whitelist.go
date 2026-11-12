package dao

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/web3chat/txchat/dtalk/pkg/mysql"
	"github.com/web3chat/txchat/dtalk/service/chatadmin/model/biz"
)

const (
	// 白名单相关SQL
	_GetWhitelistById      = `SELECT * FROM group_create_whitelist WHERE id=?`
	_GetWhitelistByAddress = `SELECT * FROM group_create_whitelist WHERE address=?`
	_GetWhitelistList      = `SELECT * FROM group_create_whitelist WHERE 1=1`
	_InsertWhitelist       = `INSERT INTO group_create_whitelist (address, remark, status, max_members, max_groups, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_UpdateWhitelist       = `UPDATE group_create_whitelist SET remark=?, status=?, max_members=?, max_groups=?, updated_at=? WHERE id=?`
	_DeleteWhitelist       = `DELETE FROM group_create_whitelist WHERE id=?`
	_GetWhitelistStats     = `SELECT 
		COUNT(*) as total,
		SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END) as active,
		SUM(CASE WHEN status = 0 THEN 1 ELSE 0 END) as disabled
		FROM group_create_whitelist`
)

// WhitelistDAO 白名单数据访问接口
type WhitelistDAO interface {
	GetWhitelistById(ctx context.Context, id *big.Int) (*biz.WhitelistInfo, error)
	GetWhitelistByAddress(ctx context.Context, address string) (*biz.WhitelistInfo, error)
	GetWhitelistList(ctx context.Context, page, pageSize int32, keyword string, status int32) ([]*biz.WhitelistInfo, int64, error)
	AddWhitelist(ctx context.Context, whitelist *biz.WhitelistInfo) error
	UpdateWhitelist(ctx context.Context, whitelist *biz.WhitelistInfo) error
	DeleteWhitelist(ctx context.Context, id *big.Int) error
	GetWhitelistStats(ctx context.Context) (*biz.WhitelistStats, error)
}

type whitelistDAO struct {
	mysql *mysql.MysqlConn
}

// NewWhitelistDAO 创建白名单DAO
func NewWhitelistDAO(mysql *mysql.MysqlConn) WhitelistDAO {
	return &whitelistDAO{
		mysql: mysql,
	}
}

// GetWhitelistById 根据ID获取白名单信息
func (d *whitelistDAO) GetWhitelistById(ctx context.Context, id *big.Int) (*biz.WhitelistInfo, error) {
	records, err := d.mysql.Query(_GetWhitelistById, id.String())
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("whitelist not found")
	}

	record := records[0]
	whitelist := d.convertRecordToBiz(record)
	return whitelist, nil
}

// GetWhitelistByAddress 根据地址获取白名单信息
func (d *whitelistDAO) GetWhitelistByAddress(ctx context.Context, address string) (*biz.WhitelistInfo, error) {
	records, err := d.mysql.Query(_GetWhitelistByAddress, address)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("whitelist not found")
	}

	record := records[0]
	whitelist := d.convertRecordToBiz(record)
	return whitelist, nil
}

// GetWhitelistList 获取白名单列表
func (d *whitelistDAO) GetWhitelistList(ctx context.Context, page, pageSize int32, keyword string, status int32) ([]*biz.WhitelistInfo, int64, error) {
	var conditions []string
	var args []interface{}

	// 构建查询条件
	if keyword != "" {
		conditions = append(conditions, "(address LIKE ? OR remark LIKE ?)")
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}
	if status >= 0 {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	// 查询总数
	countSQL := _GetWhitelistList + whereClause
	countRecords, err := d.mysql.Query(countSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	total := int64(len(countRecords))

	// 查询列表
	offset := (page - 1) * pageSize
	listSQL := _GetWhitelistList + whereClause + " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)

	records, err := d.mysql.Query(listSQL, args...)
	if err != nil {
		return nil, 0, err
	}

	var whitelists []*biz.WhitelistInfo
	for _, record := range records {
		whitelist := d.convertRecordToBiz(record)
		whitelists = append(whitelists, whitelist)
	}

	return whitelists, total, nil
}

// AddWhitelist 添加白名单
func (d *whitelistDAO) AddWhitelist(ctx context.Context, whitelist *biz.WhitelistInfo) error {
	now := time.Now()
	_, _, err := d.mysql.Exec(_InsertWhitelist,
		whitelist.Address, whitelist.Remark, whitelist.Status, whitelist.MaxMembers, whitelist.MaxGroups,
		whitelist.CreatedBy, now, now,
	)
	return err
}

// UpdateWhitelist 更新白名单
func (d *whitelistDAO) UpdateWhitelist(ctx context.Context, whitelist *biz.WhitelistInfo) error {
	_, _, err := d.mysql.Exec(_UpdateWhitelist,
		whitelist.Remark, whitelist.Status, whitelist.MaxMembers, whitelist.MaxGroups, time.Now(), whitelist.Id.String(),
	)
	return err
}

// DeleteWhitelist 删除白名单
func (d *whitelistDAO) DeleteWhitelist(ctx context.Context, id *big.Int) error {
	_, _, err := d.mysql.Exec(_DeleteWhitelist, id.String())
	return err
}

// GetWhitelistStats 获取白名单统计信息
func (d *whitelistDAO) GetWhitelistStats(ctx context.Context) (*biz.WhitelistStats, error) {
	records, err := d.mysql.Query(_GetWhitelistStats)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return &biz.WhitelistStats{}, nil
	}

	record := records[0]
	stats := &biz.WhitelistStats{}

	if total, ok := record["total"]; ok {
		stats.TotalWhitelist, _ = strconv.ParseInt(total, 10, 64)
	}
	if active, ok := record["active"]; ok {
		stats.ActiveWhitelist, _ = strconv.ParseInt(active, 10, 64)
	}
	if disabled, ok := record["disabled"]; ok {
		stats.DisabledWhitelist, _ = strconv.ParseInt(disabled, 10, 64)
	}

	return stats, nil
}

// convertRecordToBiz 转换记录为业务模型
func (d *whitelistDAO) convertRecordToBiz(record map[string]string) *biz.WhitelistInfo {
	whitelist := &biz.WhitelistInfo{}

	if id, ok := record["id"]; ok {
		whitelist.Id = new(big.Int)
		whitelist.Id.SetString(id, 10)
	}
	if address, ok := record["address"]; ok {
		whitelist.Address = address
	}
	if remark, ok := record["remark"]; ok {
		whitelist.Remark = remark
	}
	if status, ok := record["status"]; ok {
		if s, err := strconv.ParseInt(status, 10, 32); err == nil {
			whitelist.Status = int32(s)
		}
	}
	if maxMembers, ok := record["max_members"]; ok {
		if max, err := strconv.ParseInt(maxMembers, 10, 32); err == nil {
			whitelist.MaxMembers = int32(max)
		}
	}
	if maxGroups, ok := record["max_groups"]; ok {
		if max, err := strconv.ParseInt(maxGroups, 10, 32); err == nil {
			whitelist.MaxGroups = int32(max)
		}
	}
	if createdBy, ok := record["created_by"]; ok {
		whitelist.CreatedBy = createdBy
	}
	if createdAt, ok := record["created_at"]; ok {
		whitelist.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	}
	if updatedAt, ok := record["updated_at"]; ok {
		whitelist.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	}

	return whitelist
}
