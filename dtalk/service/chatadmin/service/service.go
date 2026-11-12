package service

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/web3chat/txchat/dtalk/pkg/mysql"
	"github.com/web3chat/txchat/dtalk/pkg/redis"
	"github.com/web3chat/txchat/dtalk/service/chatadmin/config"
	"github.com/web3chat/txchat/dtalk/service/chatadmin/dao"
	"github.com/web3chat/txchat/dtalk/service/chatadmin/model/biz"
)

// Service 群组管理服务
type Service struct {
	conf           *config.Config
	mysql          *mysql.MysqlConn
	mysqlRecord    *mysql.MysqlConn // dtalk_record 数据库连接
	redis          *redis.Pool
	groupInfoDAO   dao.GroupInfoDAO
	whitelistDAO   dao.WhitelistDAO
	authDAO        dao.AuthDAO
	userLoginDAO   dao.UserLoginDAO
	dailyActiveDAO dao.DailyActiveDAO
	auditDAO       dao.AdminAuditDAO
	jwtSecret      []byte
	stopRecorder   context.CancelFunc
}

// GetConfig 获取配置信息
func (s *Service) GetConfig() *config.Config {
	return s.conf
}

// New 创建新的服务实例
func New(conf *config.Config) *Service {
	// 初始化MySQL连接（dtalk 数据库）
	mysqlClient, err := mysql.NewMysqlConn(
		conf.MySQL.Host,
		fmt.Sprintf("%d", conf.MySQL.Port),
		conf.MySQL.User,
		conf.MySQL.Pwd,
		conf.MySQL.Db,
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to MySQL (dtalk): %v", err))
	}

	// 初始化MySQL连接（dtalk_record 数据库）
	var mysqlRecordClient *mysql.MysqlConn
	if conf.MySQLRecord != nil {
		mysqlRecordClient, err = mysql.NewMysqlConn(
			conf.MySQLRecord.Host,
			fmt.Sprintf("%d", conf.MySQLRecord.Port),
			conf.MySQLRecord.User,
			conf.MySQLRecord.Pwd,
			conf.MySQLRecord.Db,
		)
		if err != nil {
			panic(fmt.Sprintf("Failed to connect to MySQL (dtalk_record): %v", err))
		}
	} else {
		// 如果没有配置 dtalk_record，使用默认配置（与 dtalk 相同，但数据库名不同）
		mysqlRecordClient, err = mysql.NewMysqlConn(
			conf.MySQL.Host,
			fmt.Sprintf("%d", conf.MySQL.Port),
			conf.MySQL.User,
			conf.MySQL.Pwd,
			"dtalk_record",
		)
		if err != nil {
			panic(fmt.Sprintf("Failed to connect to MySQL (dtalk_record): %v", err))
		}
	}

	// 初始化Redis连接
	redisClient := redis.New(conf.Redis)

	// user_bindings：默认与 dtalk 同库；若配置了 MySQLAuthKey（如正式环境 authkey 独立库）则使用独立连接
	mysqlBindingsConn := mysqlClient
	if conf.MySQLAuthKey != nil {
		mysqlBindingsConn, err = mysql.NewMysqlConn(
			conf.MySQLAuthKey.Host,
			fmt.Sprintf("%d", conf.MySQLAuthKey.Port),
			conf.MySQLAuthKey.User,
			conf.MySQLAuthKey.Pwd,
			conf.MySQLAuthKey.Db,
		)
		if err != nil {
			panic(fmt.Sprintf("Failed to connect to MySQL (user_bindings / MySQLAuthKey Db=%s): %v", conf.MySQLAuthKey.Db, err))
		}
	}

	// 初始化DAO
	groupInfoDAO := dao.NewGroupInfoDAO(mysqlClient, mysqlRecordClient)
	whitelistDAO := dao.NewWhitelistDAO(mysqlClient)
	authDAO := dao.NewAuthDAO(mysqlClient)
	userLoginDAO := dao.NewUserLoginDAO(mysqlClient, mysqlRecordClient, mysqlBindingsConn)
	dailyActiveDAO := dao.NewDailyActiveDAO(mysqlClient)
	auditDAO := dao.NewAdminAuditDAO(mysqlRecordClient)
	// 确保表存在
	_ = authDAO.EnsureTables(context.Background())
	if err := dailyActiveDAO.EnsureTable(context.Background()); err != nil {
		fmt.Printf("[Service] EnsureDailyActiveTable failed: err=%v\n", err)
	}
	if err := auditDAO.EnsureTable(context.Background()); err != nil {
		fmt.Printf("[Service] EnsureAdminAuditTable failed: err=%v\n", err)
	}

	s := &Service{
		conf:           conf,
		mysql:          mysqlClient,
		mysqlRecord:    mysqlRecordClient,
		redis:          redisClient,
		groupInfoDAO:   groupInfoDAO,
		whitelistDAO:   whitelistDAO,
		authDAO:        authDAO,
		userLoginDAO:   userLoginDAO,
		dailyActiveDAO: dailyActiveDAO,
		auditDAO:       auditDAO,
		jwtSecret:      []byte("dtalk-chatadmin-secret"),
	}
	if conf.Auth != nil && conf.Auth.JwtSecret != "" {
		s.jwtSecret = []byte(conf.Auth.JwtSecret)
	}
	recorderCtx, cancel := context.WithCancel(context.Background())
	s.stopRecorder = cancel
	s.StartDailyActiveRecorder(recorderCtx)
	s.StartLoginLogRetention(recorderCtx)
	return s
}

// Close 停止后台任务
func (s *Service) Close() {
	if s.stopRecorder != nil {
		s.stopRecorder()
	}
}

// GetAuthNonce 生成一次性nonce。地址必须是已启用的管理员白名单。
func (s *Service) GetAuthNonce(ctx context.Context, address string) (string, int64, error) {
	addr := strings.ToLower(strings.TrimSpace(address))
	if !isEthAddress(addr) {
		fmt.Printf("[Service] GetAuthNonce rejected: invalid address=%q\n", address)
		return "", 0, fmt.Errorf("invalid address")
	}
	allowed, err := s.authDAO.IsAddressAllowed(ctx, addr)
	if err != nil {
		fmt.Printf("[Service] GetAuthNonce whitelist query failed: address=%s err=%v\n", addr, err)
		return "", 0, err
	}
	if !allowed {
		fmt.Printf("[Service] GetAuthNonce rejected: address=%s not in whitelist\n", addr)
		return "", 0, fmt.Errorf("address not allowed")
	}
	nonce := fmt.Sprintf("DTalk Admin Login:%d", time.Now().UnixNano())
	expire := time.Now().Add(5 * time.Minute).UnixMilli()
	if err := s.authDAO.UpsertNonce(ctx, &dao.AuthNonce{Address: addr, Nonce: nonce, ExpireAt: expire}); err != nil {
		return "", 0, err
	}
	return nonce, expire, nil
}

// Config 获取配置
func (s *Service) Config() *config.Config {
	return s.conf
}

// GetGroupInfo 获取群组信息
func (s *Service) GetGroupInfo(ctx context.Context, groupId *big.Int) (*biz.GroupInfo, error) {
	return s.groupInfoDAO.GetGroupInfo(ctx, groupId)
}

// GetGroupList 获取群组列表
func (s *Service) GetGroupList(ctx context.Context, req *biz.GroupListRequest) (*biz.GroupListResponse, error) {
	groups, total, err := s.groupInfoDAO.GetGroupList(ctx, req.Page, req.PageSize, req.Keyword, req.Status)
	if err != nil {
		return nil, err
	}

	// 转换指针切片为值切片
	groupList := make([]biz.GroupInfo, len(groups))
	for i, group := range groups {
		groupList[i] = *group
	}

	return &biz.GroupListResponse{
		Total: total,
		List:  groupList,
	}, nil
}

// UpdateGroupMaximum 更新群组最大人数
func (s *Service) UpdateGroupMaximum(ctx context.Context, groupId *big.Int, maximum int32) error {
	return s.groupInfoDAO.UpdateGroupMaximum(ctx, groupId, maximum)
}

// UpdateGroupStatus 更新群组状态
func (s *Service) UpdateGroupStatus(ctx context.Context, groupId *big.Int, status int32) error {
	return s.groupInfoDAO.UpdateGroupStatus(ctx, groupId, status)
}

// UpdateGroupName 更新群组名称
func (s *Service) UpdateGroupName(ctx context.Context, groupId *big.Int, name string) error {
	return s.groupInfoDAO.UpdateGroupName(ctx, groupId, name)
}

// GetGroupStats 获取群组统计信息
func (s *Service) GetGroupStats(ctx context.Context) (*biz.GroupStats, error) {
	stats, err := s.groupInfoDAO.GetGroupStats(ctx)
	if err != nil {
		return nil, err
	}
	// 今日活跃查失败时记日志，对应指标保持 0，不让整页统计失败。
	if activeUsers, err := s.groupInfoDAO.GetTodayActiveUserCount(ctx); err != nil {
		fmt.Printf("[Service] GetGroupStats today active users failed: err=%v\n", err)
	} else {
		stats.TodayActiveUsers = activeUsers
	}
	if activeGroups, err := s.groupInfoDAO.GetTodayActiveGroupCount(ctx); err != nil {
		fmt.Printf("[Service] GetGroupStats today active groups failed: err=%v\n", err)
	} else {
		stats.TodayActiveGroups = activeGroups
	}
	return stats, nil
}

// GetGroupMembers 分页获取群成员列表
func (s *Service) GetGroupMembers(ctx context.Context, groupId *big.Int, page, pageSize int32) ([]*biz.GroupMember, int64, error) {
	return s.groupInfoDAO.GetGroupMembers(ctx, groupId, page, pageSize)
}

// GetGroupsByOwner 根据创建者地址获取群组列表
func (s *Service) GetGroupsByOwner(ctx context.Context, ownerId string, page, pageSize int32) (*biz.GroupListResponse, error) {
	groups, total, err := s.groupInfoDAO.GetGroupsByOwner(ctx, ownerId, page, pageSize)
	if err != nil {
		return nil, err
	}

	// 转换指针切片为值切片
	groupList := make([]biz.GroupInfo, len(groups))
	for i, group := range groups {
		groupList[i] = *group
	}

	return &biz.GroupListResponse{
		Total: total,
		List:  groupList,
	}, nil
}

// GetGroupCountByOwner 根据创建者地址获取群组数量
func (s *Service) GetGroupCountByOwner(ctx context.Context, ownerId string) (int64, error) {
	return s.groupInfoDAO.GetGroupCountByOwner(ctx, ownerId)
}

// GetTodayGroupCount 获取今日创建的群聊数量
func (s *Service) GetTodayGroupCount(ctx context.Context) (int64, error) {
	return s.groupInfoDAO.GetTodayGroupCount(ctx)
}

// GetTotalUserCount 读取日活快照里的累计地址数，避免刷新时扫信号全表。
func (s *Service) GetTotalUserCount(ctx context.Context) (int64, error) {
	today := time.Now().Format("2006-01-02")
	rec, err := s.snapshotByDate(ctx, today)
	if err != nil {
		fmt.Printf("[Service] GetTotalUserCount snapshot failed: date=%s err=%v\n", today, err)
		return 0, err
	}
	if rec != nil {
		return rec.TotalUsers, nil
	}
	latest, err := s.latestSnapshot(ctx)
	if err != nil {
		fmt.Printf("[Service] GetTotalUserCount latest snapshot failed: err=%v\n", err)
		return 0, err
	}
	if latest == nil {
		fmt.Printf("[Service] GetTotalUserCount no snapshot yet: date=%s\n", today)
		return 0, nil
	}
	return latest.TotalUsers, nil
}

// GetTodayNewUserCount 用今天和昨天快照的累计地址数相减，得到今日新增。
func (s *Service) GetTodayNewUserCount(ctx context.Context) (int64, error) {
	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	rows, err := s.snapshotsByDates(ctx, []string{today, yesterday})
	if err != nil {
		fmt.Printf("[Service] GetTodayNewUserCount failed: today=%s yesterday=%s err=%v\n", today, yesterday, err)
		return 0, err
	}
	todayRec := rows[today]
	yesterdayRec := rows[yesterday]
	var todayTotal, yesterdayTotal int64
	if todayRec != nil {
		todayTotal = todayRec.TotalUsers
	}
	if yesterdayRec != nil {
		yesterdayTotal = yesterdayRec.TotalUsers
	}
	count := newUsersFromSnapshots(todayTotal, yesterdayTotal, todayRec != nil, yesterdayRec != nil)
	if todayRec == nil || yesterdayRec == nil {
		fmt.Printf("[Service] GetTodayNewUserCount snapshot incomplete: today=%s hasToday=%t yesterday=%s hasYesterday=%t\n",
			today, todayRec != nil, yesterday, yesterdayRec != nil)
	}
	return count, nil
}

func (s *Service) snapshotByDate(ctx context.Context, date string) (*biz.DailyActiveRecord, error) {
	rows, err := s.snapshotsByDates(ctx, []string{date})
	if err != nil {
		return nil, err
	}
	return rows[date], nil
}

func (s *Service) latestSnapshot(ctx context.Context) (*biz.DailyActiveRecord, error) {
	list, _, err := s.dailyActiveDAO.List(ctx, "", "", nil, 1, 1)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return list[0], nil
}

func (s *Service) snapshotsByDates(ctx context.Context, dates []string) (map[string]*biz.DailyActiveRecord, error) {
	list, _, err := s.dailyActiveDAO.List(ctx, "", "", dates, 1, int32(len(dates)))
	if err != nil {
		return nil, err
	}
	out := make(map[string]*biz.DailyActiveRecord, len(list))
	for _, rec := range list {
		if rec != nil {
			out[rec.StatDate] = rec
		}
	}
	return out, nil
}

// newUsersFromSnapshots 今天累计减昨天累计。缺快照或出现回退时记 0。
func newUsersFromSnapshots(today, yesterday int64, hasToday, hasYesterday bool) int64 {
	if !hasToday || !hasYesterday || today < yesterday {
		return 0
	}
	return today - yesterday
}

// GetWhitelistById 根据ID获取白名单信息
func (s *Service) GetWhitelistById(ctx context.Context, id *big.Int) (*biz.WhitelistInfo, error) {
	return s.whitelistDAO.GetWhitelistById(ctx, id)
}

// GetWhitelistByAddress 根据地址获取白名单信息
func (s *Service) GetWhitelistByAddress(ctx context.Context, address string) (*biz.WhitelistInfo, error) {
	return s.whitelistDAO.GetWhitelistByAddress(ctx, address)
}

// GetWhitelistList 获取白名单列表
func (s *Service) GetWhitelistList(ctx context.Context, req *biz.WhitelistListRequest) (*biz.WhitelistListResponse, error) {
	whitelists, total, err := s.whitelistDAO.GetWhitelistList(ctx, req.Page, req.PageSize, req.Keyword, req.Status)
	if err != nil {
		return nil, err
	}

	// 转换指针切片为值切片
	whitelistList := make([]biz.WhitelistInfo, len(whitelists))
	for i, whitelist := range whitelists {
		whitelistList[i] = *whitelist
	}

	return &biz.WhitelistListResponse{
		Total: total,
		List:  whitelistList,
	}, nil
}

// AddWhitelist 添加白名单
func (s *Service) AddWhitelist(ctx context.Context, req *biz.AddWhitelistRequest) error {
	whitelist := &biz.WhitelistInfo{
		Address:    req.Address,
		Remark:     req.Remark,
		Status:     1, // 默认启用
		MaxMembers: req.MaxMembers,
		MaxGroups:  req.MaxGroups,
		CreatedBy:  req.CreatedBy,
	}
	return s.whitelistDAO.AddWhitelist(ctx, whitelist)
}

// UpdateWhitelist 更新白名单
func (s *Service) UpdateWhitelist(ctx context.Context, req *biz.UpdateWhitelistRequest) error {
	whitelist := &biz.WhitelistInfo{
		Id:         req.Id,
		Remark:     req.Remark,
		Status:     req.Status,
		MaxMembers: req.MaxMembers,
		MaxGroups:  req.MaxGroups,
	}
	return s.whitelistDAO.UpdateWhitelist(ctx, whitelist)
}

// DeleteWhitelist 删除白名单
func (s *Service) DeleteWhitelist(ctx context.Context, req *biz.DeleteWhitelistRequest) error {
	return s.whitelistDAO.DeleteWhitelist(ctx, req.Id)
}

// GetWhitelistStats 获取白名单统计信息
func (s *Service) GetWhitelistStats(ctx context.Context) (*biz.WhitelistStats, error) {
	return s.whitelistDAO.GetWhitelistStats(ctx)
}

// VerifyAndIssueToken 校验 personal_sign 并签发 token。
// 签名必须能恢复出与请求相同的地址，且该地址在启用的白名单中。
func (s *Service) VerifyAndIssueToken(ctx context.Context, address string, signature string) (string, int64, error) {
	addr := strings.ToLower(strings.TrimSpace(address))
	signature = strings.TrimSpace(signature)
	if !isEthAddress(addr) || signature == "" {
		fmt.Printf("[Service] VerifyAndIssueToken rejected: invalid params address=%q signatureEmpty=%v\n", address, signature == "")
		return "", 0, fmt.Errorf("invalid params")
	}
	n, err := s.authDAO.GetNonce(ctx, addr)
	if err != nil || n == nil || n.Nonce == "" {
		fmt.Printf("[Service] VerifyAndIssueToken nonce missing: address=%s err=%v\n", addr, err)
		return "", 0, fmt.Errorf("nonce not found")
	}
	if time.Now().UnixMilli() > n.ExpireAt {
		_ = s.authDAO.DeleteNonce(ctx, addr)
		fmt.Printf("[Service] VerifyAndIssueToken nonce expired: address=%s expireAt=%d\n", addr, n.ExpireAt)
		return "", 0, fmt.Errorf("nonce expired")
	}
	recovered, err := recoverPersonalSignAddress(n.Nonce, signature)
	if err != nil {
		fmt.Printf("[Service] VerifyAndIssueToken signature invalid: address=%s err=%v\n", addr, err)
		return "", 0, fmt.Errorf("invalid signature")
	}
	if recovered != addr {
		fmt.Printf("[Service] VerifyAndIssueToken address mismatch: claimed=%s recovered=%s\n", addr, recovered)
		return "", 0, fmt.Errorf("signature address mismatch")
	}

	allowed, err := s.authDAO.IsAddressAllowed(ctx, addr)
	if err != nil {
		fmt.Printf("[Service] VerifyAndIssueToken whitelist query failed: address=%s err=%v\n", addr, err)
		return "", 0, err
	}
	if !allowed {
		return "", 0, fmt.Errorf("address not allowed")
	}
	// 颁发JWT，TTL 从配置读取
	ttl := 2 * time.Hour
	if s.conf.Auth != nil && s.conf.Auth.JwtTTLSeconds > 0 {
		ttl = time.Duration(s.conf.Auth.JwtTTLSeconds) * time.Second
	}
	tokenStr, exp, err := s.GenerateJWT(addr, ttl)
	if err != nil {
		return "", 0, err
	}
	// 使用后删除nonce
	_ = s.authDAO.DeleteNonce(ctx, addr)
	return tokenStr, exp, nil
}

// CustomClaims 自定义JWT载荷
type CustomClaims struct {
	Address string `json:"address"`
	jwt.RegisteredClaims
}

// GenerateJWT 签发JWT
func (s *Service) GenerateJWT(address string, ttl time.Duration) (string, int64, error) {
	now := time.Now()
	exp := now.Add(ttl)
	claims := &CustomClaims{
		Address: address,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "dtalk-chatadmin",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			Subject:   address,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", 0, err
	}
	return signed, exp.Unix(), nil
}

// ValidateJWT 验证JWT
func (s *Service) ValidateJWT(tokenStr string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("invalid token")
}

// GetAdminLevel 获取管理员级别
// 返回: 0=普通管理员, 1=超级管理员, -1=未找到或已禁用, error
func (s *Service) GetAdminLevel(ctx context.Context, address string) (int32, error) {
	return s.authDAO.GetAdminLevel(ctx, address)
}

// IsSuperAdmin 检查是否为超级管理员
func (s *Service) IsSuperAdmin(ctx context.Context, address string) (bool, error) {
	level, err := s.GetAdminLevel(ctx, address)
	if err != nil {
		return false, err
	}
	return level == 1, nil
}

// GetUserLoginHistory 根据用户地址查询登录历史，并关联账号绑定信息。
// 绑定表查询失败时只记日志，登录记录仍返回。
func (s *Service) GetUserLoginHistory(ctx context.Context, req *biz.UserLoginHistoryRequest) (*biz.UserLoginWithBinding, error) {
	req.Address = strings.TrimSpace(req.Address)
	if req.Address == "" {
		return nil, fmt.Errorf("address is required")
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	logs, total, err := s.userLoginDAO.GetLoginHistory(ctx, req.Address, req.Page, req.PageSize, req.StartTime, req.EndTime)
	if err != nil {
		fmt.Printf("[Service] GetUserLoginHistory logs failed: address=%s page=%d pageSize=%d startTime=%d endTime=%d err=%v\n",
			req.Address, req.Page, req.PageSize, req.StartTime, req.EndTime, err)
		return nil, err
	}

	binding, err := s.userLoginDAO.GetUserBinding(ctx, req.Address)
	if err != nil {
		fmt.Printf("[Service] GetUserLoginHistory binding failed: address=%s err=%v\n", req.Address, err)
		binding = nil
	}

	return &biz.UserLoginWithBinding{
		Address:  req.Address,
		Binding:  binding,
		Logs:     logs,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// RecordAudit 写入一条操作留痕。写入失败只记日志，不打断原操作。
func (s *Service) RecordAudit(ctx context.Context, rec *biz.AdminAudit) {
	if s.auditDAO == nil || rec == nil {
		fmt.Printf("[Service] RecordAudit skipped: dao or record nil action=%v\n", rec)
		return
	}
	if err := s.auditDAO.Insert(ctx, rec); err != nil {
		fmt.Printf("[Service] RecordAudit failed: operator=%s action=%s targetType=%s targetId=%s err=%v\n",
			rec.Operator, rec.Action, rec.TargetType, rec.TargetID, err)
	}
}

// ListAudit 分页查询操作留痕。action、targetType 支持多个值，条件之间是交集。
func (s *Service) ListAudit(ctx context.Context, actions, targetTypes []string, targetID, operator string, page, pageSize int32) (*biz.AdminAuditList, error) {
	if s.auditDAO == nil {
		return nil, fmt.Errorf("audit dao is nil")
	}
	list, total, err := s.auditDAO.List(ctx, actions, targetTypes, targetID, operator, page, pageSize)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []*biz.AdminAudit{}
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
	return &biz.AdminAuditList{List: list, Total: total, Page: page, PageSize: pageSize}, nil
}

// GetAllLoginLogs 分页查询全站用户登录日志（可选按多个 uid 并行筛选）
func (s *Service) GetAllLoginLogs(ctx context.Context, req *biz.AllLoginLogsRequest) (*biz.AllLoginLogsResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	logs, total, err := s.userLoginDAO.ListAllLoginLogs(ctx, page, pageSize, req.StartTime, req.EndTime, req.Uid)
	if err != nil {
		return nil, err
	}

	return &biz.AllLoginLogsResponse{
		List:     logs,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}
