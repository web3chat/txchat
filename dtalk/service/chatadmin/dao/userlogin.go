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
	_GetUserBindingByAddr = `SELECT id, phone, wechat_id, xiaohongshu_id, chataddress, is_deleted, created_at, updated_at FROM user_bindings WHERE chataddress = ? AND is_deleted = 0 LIMIT 1`

	maxLoginLogUidFilters = 50 // QueryArray uid 上限，防止超长 IN 列表
	loginLogPurgeBatch    = 2000
	loginLogPurgeBatches  = 200
)

// UserLoginDAO 用户登录日志与账号绑定数据访问接口
type UserLoginDAO interface {
	GetLoginHistory(ctx context.Context, address string, page, pageSize int32, startTime, endTime int64) ([]*biz.LoginRecord, int64, error)
	ListAllLoginLogs(ctx context.Context, page, pageSize int32, startTime, endTime int64, uidFilters []string) ([]*biz.LoginRecord, int64, error)
	DeleteLoginLogsBefore(ctx context.Context, beforeMilli int64) (int64, error)
	GetUserBinding(ctx context.Context, address string) (*biz.UserBinding, error)
}

type userLoginDAO struct {
	mysqlBiz      *mysql.MysqlConn // dtalk（与其它 DAO 一致）
	mysqlRecord   *mysql.MysqlConn // dtalk_record：dtalk_login_log
	mysqlBindings *mysql.MysqlConn // user_bindings：可与 mysqlBiz 同一连接，或通过配置指向 authkey 等独立库
}

// NewUserLoginDAO 创建用户登录 DAO：登录日志走 mysqlRecord；账号绑定走 mysqlBindings（通常为 dtalk，可与 mysqlBiz 相同指针）
func NewUserLoginDAO(mysqlBiz, mysqlRecord, mysqlBindings *mysql.MysqlConn) UserLoginDAO {
	if mysqlBindings == nil {
		mysqlBindings = mysqlBiz
	}
	return &userLoginDAO{mysqlBiz: mysqlBiz, mysqlRecord: mysqlRecord, mysqlBindings: mysqlBindings}
}

func clampLoginListPagination(page, pageSize int32) (int32, int32) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func normalizeDistinctUIDFilters(uidFilters []string) []string {
	if len(uidFilters) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(uidFilters))
	out := make([]string, 0, len(uidFilters))
	for _, raw := range uidFilters {
		u := strings.TrimSpace(raw)
		if u == "" {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
		if len(out) >= maxLoginLogUidFilters {
			break
		}
	}
	return out
}

// GetLoginHistory 分页查询用户登录记录（支持时间范围过滤）
func (d *userLoginDAO) GetLoginHistory(ctx context.Context, address string, page, pageSize int32, startTime, endTime int64) ([]*biz.LoginRecord, int64, error) {
	var conditions []string
	var args []interface{}

	conditions = append(conditions, "uid = ?")
	args = append(args, address)

	if startTime > 0 {
		conditions = append(conditions, "login_at >= ?")
		args = append(args, startTime)
	}
	if endTime > 0 {
		conditions = append(conditions, "login_at <= ?")
		args = append(args, endTime)
	}

	where := " WHERE " + strings.Join(conditions, " AND ")

	// 查询总数
	countSQL := "SELECT COUNT(*) as count FROM dtalk_login_log" + where
	countRecords, err := d.mysqlRecord.Query(countSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询登录日志总数失败: %w", err)
	}
	var total int64
	if len(countRecords) > 0 {
		total, _ = strconv.ParseInt(countRecords[0]["count"], 10, 64)
	}

	page, pageSize = clampLoginListPagination(page, pageSize)
	offset := (page - 1) * pageSize

	listSQL := "SELECT id, uid, client_ip, login_at, device_type, device_uuid, device_name, country, city_region, isp, created_at FROM dtalk_login_log" +
		where + " ORDER BY login_at DESC LIMIT ? OFFSET ?"
	listArgs := append(args, pageSize, offset)

	records, err := d.mysqlRecord.Query(listSQL, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询登录日志列表失败: %w", err)
	}

	logs := make([]*biz.LoginRecord, 0, len(records))
	for _, r := range records {
		logs = append(logs, d.convertLoginRecord(r))
	}

	return logs, total, nil
}

// ListAllLoginLogs 分页查询全站登录日志；uidFilters 非空时等价于 uid IN (...) 与时间条件的交集
func (d *userLoginDAO) ListAllLoginLogs(ctx context.Context, page, pageSize int32, startTime, endTime int64, uidFilters []string) ([]*biz.LoginRecord, int64, error) {
	uids := normalizeDistinctUIDFilters(uidFilters)

	var conditions []string
	var args []interface{}

	if len(uids) > 0 {
		ph := strings.TrimSuffix(strings.Repeat("?,", len(uids)), ",")
		conditions = append(conditions, "uid IN ("+ph+")")
		for _, u := range uids {
			args = append(args, u)
		}
	}
	if startTime > 0 {
		conditions = append(conditions, "login_at >= ?")
		args = append(args, startTime)
	}
	if endTime > 0 {
		conditions = append(conditions, "login_at <= ?")
		args = append(args, endTime)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	countSQL := "SELECT COUNT(*) as count FROM dtalk_login_log" + where
	countRecords, err := d.mysqlRecord.Query(countSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询全站登录日志总数失败: %w", err)
	}
	var total int64
	if len(countRecords) > 0 {
		total, _ = strconv.ParseInt(countRecords[0]["count"], 10, 64)
	}

	page, pageSize = clampLoginListPagination(page, pageSize)
	offset := (page - 1) * pageSize

	listSQL := "SELECT id, uid, client_ip, login_at, device_type, device_uuid, device_name, country, city_region, isp, created_at FROM dtalk_login_log" +
		where + " ORDER BY login_at DESC LIMIT ? OFFSET ?"
	listArgs := append(args, pageSize, offset)

	records, err := d.mysqlRecord.Query(listSQL, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询全站登录日志列表失败: %w", err)
	}

	logs := make([]*biz.LoginRecord, 0, len(records))
	for _, r := range records {
		logs = append(logs, d.convertLoginRecord(r))
	}

	return logs, total, nil
}

// DeleteLoginLogsBefore 分批删除 login_at 早于截止时间的登录日志。返回实际删除行数。
func (d *userLoginDAO) DeleteLoginLogsBefore(ctx context.Context, beforeMilli int64) (int64, error) {
	if d.mysqlRecord == nil {
		return 0, fmt.Errorf("dtalk_record database connection not initialized")
	}
	if beforeMilli <= 0 {
		return 0, fmt.Errorf("invalid login log cutoff: %d", beforeMilli)
	}
	var deleted int64
	for batch := 0; batch < loginLogPurgeBatches; batch++ {
		n, _, err := d.mysqlRecord.Exec(
			`DELETE FROM dtalk_login_log WHERE login_at < ? LIMIT ?`,
			beforeMilli, loginLogPurgeBatch,
		)
		if err != nil {
			fmt.Printf("[UserLoginDAO] DeleteLoginLogsBefore failed: before=%d deleted=%d batch=%d err=%v\n",
				beforeMilli, deleted, batch, err)
			return deleted, fmt.Errorf("删除过期登录日志失败: %w", err)
		}
		deleted += n
		if n < int64(loginLogPurgeBatch) {
			break
		}
	}
	fmt.Printf("[UserLoginDAO] DeleteLoginLogsBefore ok: before=%d deleted=%d\n", beforeMilli, deleted)
	return deleted, nil
}

// GetUserBinding 根据聊天地址查询用户账号绑定信息
func (d *userLoginDAO) GetUserBinding(ctx context.Context, address string) (*biz.UserBinding, error) {
	records, err := d.mysqlBindings.Query(_GetUserBindingByAddr, address)
	if err != nil {
		return nil, fmt.Errorf("查询用户绑定信息失败: %w", err)
	}
	if len(records) == 0 {
		return nil, nil
	}
	return d.convertUserBinding(records[0]), nil
}

func (d *userLoginDAO) convertLoginRecord(r map[string]string) *biz.LoginRecord {
	rec := &biz.LoginRecord{}
	if v, ok := r["id"]; ok {
		rec.Id, _ = strconv.ParseInt(v, 10, 64)
	}
	if v, ok := r["uid"]; ok {
		rec.Uid = v
	}
	if v, ok := r["client_ip"]; ok {
		rec.ClientIP = v
	}
	if v, ok := r["login_at"]; ok {
		rec.LoginAt, _ = strconv.ParseInt(v, 10, 64)
	}
	if v, ok := r["device_type"]; ok {
		rec.DeviceType = v
	}
	if v, ok := r["device_uuid"]; ok {
		rec.DeviceUUID = v
	}
	if v, ok := r["device_name"]; ok {
		rec.DeviceName = v
	}
	if v, ok := r["country"]; ok {
		rec.Country = v
	}
	if v, ok := r["city_region"]; ok {
		rec.CityRegion = v
	}
	if v, ok := r["isp"]; ok {
		rec.ISP = v
	}
	if v, ok := r["created_at"]; ok {
		rec.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", v)
	}
	return rec
}

func (d *userLoginDAO) convertUserBinding(r map[string]string) *biz.UserBinding {
	b := &biz.UserBinding{}
	if v, ok := r["id"]; ok {
		b.Id, _ = strconv.ParseInt(v, 10, 64)
	}
	if v, ok := r["phone"]; ok {
		b.Phone = v
	}
	if v, ok := r["wechat_id"]; ok {
		b.WechatId = v
	}
	if v, ok := r["xiaohongshu_id"]; ok {
		b.XiaohongshuId = v
	}
	if v, ok := r["chataddress"]; ok {
		b.ChatAddress = v
	}
	if v, ok := r["is_deleted"]; ok {
		n, _ := strconv.ParseInt(v, 10, 32)
		b.IsDeleted = int32(n)
	}
	if v, ok := r["created_at"]; ok {
		b.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", v)
	}
	if v, ok := r["updated_at"]; ok {
		b.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", v)
	}
	return b
}
