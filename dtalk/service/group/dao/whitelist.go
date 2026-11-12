package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/web3chat/txchat/dtalk/pkg/util"
	"github.com/web3chat/txchat/dtalk/service/group/model"
)

// WhitelistDAO 白名单数据访问接口
type WhitelistDAO struct {
	dao *Dao
}

// NewWhitelistDAO 创建白名单DAO
func NewWhitelistDAO(dao *Dao) *WhitelistDAO {
	return &WhitelistDAO{
		dao: dao,
	}
}

// 白名单相关SQL语句
const (
	_CheckWhitelistWithLimit = `
SELECT 
    address,
    max_members,
    max_groups,
    status,
    remark,
    created_at,
    updated_at
FROM group_create_whitelist 
WHERE address = ? AND status = 1
LIMIT 1
`

	_AddWhitelist = `
INSERT INTO group_create_whitelist 
(address, remark, status, max_members, created_by, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
`

	_UpdateWhitelist = `
UPDATE group_create_whitelist 
SET remark = ?, status = ?, max_members = ?, updated_at = ?
WHERE address = ?
`

	_DeleteWhitelist = `
UPDATE group_create_whitelist 
SET status = 0, updated_at = ?
WHERE address = ?
`

	_GetWhitelistList = `
SELECT 
    id,
    address,
    remark,
    status,
    max_members,
    created_by,
    created_at,
    updated_at
FROM group_create_whitelist 
WHERE status = 1
ORDER BY created_at DESC
LIMIT ?, ?
`
)

// CheckWhitelistWithLimit 检查地址是否在白名单中，并获取人数上限
func (w *WhitelistDAO) CheckWhitelistWithLimit(ctx context.Context, address string) (*model.CheckWhitelistResp, error) {
	if address == "" {
		return &model.CheckWhitelistResp{
			IsInWhitelist: false,
			MaxMembers:    0,
			Address:       address,
		}, nil
	}

	maps, err := w.dao.conn.Query(_CheckWhitelistWithLimit, address)
	if err != nil {
		return nil, fmt.Errorf("查询白名单失败: %w", err)
	}

	if len(maps) == 0 {
		// 不在白名单中
		return &model.CheckWhitelistResp{
			IsInWhitelist: false,
			MaxMembers:    0,
			Address:       address,
		}, nil
	}

	res := maps[0]
	maxMembers := util.ToInt32(res["max_members"])
	maxGroups := util.ToInt32(res["max_groups"])

	return &model.CheckWhitelistResp{
		IsInWhitelist: true,
		MaxMembers:    maxMembers,
		MaxGroups:     maxGroups,
		Address:       address,
	}, nil
}

// AddWhitelist 添加白名单记录
func (w *WhitelistDAO) AddWhitelist(ctx context.Context, info *model.WhitelistInfo) error {
	nowTime := time.Now().UnixNano() / 1e6

	_, _, err := w.dao.conn.Exec(_AddWhitelist,
		info.Address,
		info.Remark,
		info.Status,
		info.MaxMembers,
		info.CreatedBy,
		nowTime,
		nowTime)

	if err != nil {
		return fmt.Errorf("添加白名单记录失败: %w", err)
	}

	return nil
}

// UpdateWhitelist 更新白名单记录
func (w *WhitelistDAO) UpdateWhitelist(ctx context.Context, info *model.WhitelistInfo) error {
	nowTime := time.Now().UnixNano() / 1e6

	result, _, err := w.dao.conn.Exec(_UpdateWhitelist,
		info.Remark,
		info.Status,
		info.MaxMembers,
		nowTime,
		info.Address)

	if err != nil {
		return fmt.Errorf("更新白名单记录失败: %w", err)
	}

	affected := result
	if affected == 0 {
		return fmt.Errorf("白名单记录不存在: %s", info.Address)
	}

	return nil
}

// DeleteWhitelist 删除白名单记录（软删除）
func (w *WhitelistDAO) DeleteWhitelist(ctx context.Context, address string) error {
	nowTime := time.Now().UnixNano() / 1e6

	result, _, err := w.dao.conn.Exec(_DeleteWhitelist, nowTime, address)
	if err != nil {
		return fmt.Errorf("删除白名单记录失败: %w", err)
	}

	affected := result

	if affected == 0 {
		return fmt.Errorf("白名单记录不存在: %s", address)
	}

	return nil
}

// GetWhitelistList 获取白名单列表
func (w *WhitelistDAO) GetWhitelistList(ctx context.Context, offset, limit int) ([]*model.WhitelistInfo, error) {
	maps, err := w.dao.conn.Query(_GetWhitelistList, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("查询白名单列表失败: %w", err)
	}

	var list []*model.WhitelistInfo
	for _, res := range maps {
		info := &model.WhitelistInfo{
			ID:         util.ToInt64(res["id"]),
			Address:    util.ToString(res["address"]),
			Remark:     util.ToString(res["remark"]),
			Status:     util.ToInt32(res["status"]),
			MaxMembers: util.ToInt32(res["max_members"]),
			CreatedBy:  util.ToString(res["created_by"]),
			CreatedAt:  util.ToInt64(res["created_at"]),
			UpdatedAt:  util.ToInt64(res["updated_at"]),
		}
		list = append(list, info)
	}

	return list, nil
}
