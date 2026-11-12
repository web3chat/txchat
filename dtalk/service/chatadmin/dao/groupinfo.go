package dao

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/web3chat/txchat/dtalk/pkg/mysql"
	"github.com/web3chat/txchat/dtalk/service/chatadmin/model/biz"
)

const (
	// 群组信息相关SQL
	_GetGroupInfoByGroupId = `SELECT * FROM dtalk_group_info WHERE group_id=?`
	_GetGroupInfoList      = `SELECT * FROM dtalk_group_info`
	_GetGroupsByOwner      = `SELECT * FROM dtalk_group_info WHERE group_owner_id=?`
	_GetGroupCountByOwner  = `SELECT COUNT(*) as count FROM dtalk_group_info WHERE group_owner_id=?`
	_UpdateGroupMaximum    = `UPDATE dtalk_group_info SET group_maximum=?, group_update_time=? WHERE group_id=?`
	_UpdateGroupStatus     = `UPDATE dtalk_group_info SET group_status=?, group_update_time=? WHERE group_id=?`
	_UpdateGroupName       = `UPDATE dtalk_group_info SET group_name=?, group_update_time=? WHERE group_id=?`
	_GetGroupStats         = `SELECT 
		COUNT(*) as total,
		SUM(CASE WHEN group_status = 0 THEN 1 ELSE 0 END) as active,
		SUM(CASE WHEN group_status = 1 THEN 1 ELSE 0 END) as banned,
		SUM(CASE WHEN group_status = 2 THEN 1 ELSE 0 END) as dissolved
		FROM dtalk_group_info`
	_GetTodayGroupCount = `SELECT COUNT(*) as count FROM dtalk_group_info WHERE group_create_time >= ? AND group_create_time < ?`
	// 每日活跃用户数（今日在 dtalk_signal_content 有记录的去重 uid）
	_GetTodayActiveUserCount = `SELECT COUNT(DISTINCT uid) as count FROM dtalk_signal_content WHERE create_time >= ? AND create_time < ?`
	_ListActiveUserIDs       = `SELECT DISTINCT uid AS uid FROM dtalk_signal_content WHERE create_time >= ? AND create_time < ? ORDER BY uid LIMIT ? OFFSET ?`
	// 今日有消息的群。群消息在 dtalk_group_msg_relation，other_uid 为群 ID。
	// type=0 是发送者的发件箱，每个群消息只计一行，避免按成员行重复扫描。
	_GetTodayActiveGroupCount = `SELECT COUNT(DISTINCT other_uid) AS count FROM dtalk_group_msg_relation WHERE type = 0 AND create_time >= ? AND create_time < ?`
	_ListActiveGroupIDs       = `SELECT DISTINCT other_uid AS group_id FROM dtalk_group_msg_relation WHERE type = 0 AND create_time >= ? AND create_time < ? ORDER BY other_uid LIMIT ? OFFSET ?`
	// 使用 dtalk_signal_content 表统计用户数（更准确，反映实际活跃用户）
	_GetTotalUserCount    = `SELECT COUNT(DISTINCT uid) as count FROM dtalk_signal_content`
	_GetTotalGroupCount   = `SELECT COUNT(*) AS count FROM dtalk_group_info`
	_GetTodayNewUserCount = `SELECT COUNT(DISTINCT uid) as count FROM dtalk_signal_content WHERE create_time >= ? AND create_time < ?`
	// 群成员相关SQL。退群成员 group_member_type=10，列表不包含。
	_CountMembersByGroupId = `
SELECT COUNT(*) AS count
FROM dtalk_group_member AS mem
WHERE mem.group_id=? AND mem.group_member_type<10
`
	_ListMembersByGroupId = `
SELECT 
mem.group_id as group_id, 
mem.group_member_id as group_member_id, 
mem.group_member_type as group_member_type, 
mem.group_member_name as group_member_name, 
mem.group_member_join_time as group_member_join_time,
COALESCE(mute.group_member_mute_time, 0) as group_member_mute_time
FROM dtalk_group_member AS mem LEFT JOIN dtalk_group_member_mute AS mute 
ON mem.group_id=mute.group_id AND mem.group_member_id=mute.group_member_id
WHERE mem.group_id=? AND mem.group_member_type<10
ORDER BY mem.group_member_type DESC, mem.group_member_join_time ASC
LIMIT ? OFFSET ?
`
)

// GroupInfoDAO 群组信息数据访问接口
type GroupInfoDAO interface {
	GetGroupInfo(ctx context.Context, groupId *big.Int) (*biz.GroupInfo, error)
	GetGroupList(ctx context.Context, page, pageSize int32, keyword string, status int32) ([]*biz.GroupInfo, int64, error)
	GetGroupsByOwner(ctx context.Context, ownerId string, page, pageSize int32) ([]*biz.GroupInfo, int64, error)
	GetGroupCountByOwner(ctx context.Context, ownerId string) (int64, error)
	UpdateGroupMaximum(ctx context.Context, groupId *big.Int, maximum int32) error
	UpdateGroupStatus(ctx context.Context, groupId *big.Int, status int32) error
	UpdateGroupName(ctx context.Context, groupId *big.Int, name string) error
	GetGroupStats(ctx context.Context) (*biz.GroupStats, error)
	GetGroupMembers(ctx context.Context, groupId *big.Int, page, pageSize int32) ([]*biz.GroupMember, int64, error)
	GetTodayGroupCount(ctx context.Context) (int64, error)
	GetTodayActiveUserCount(ctx context.Context) (int64, error)
	GetTodayActiveGroupCount(ctx context.Context) (int64, error)
	GetActiveUserCountByDay(ctx context.Context, day time.Time) (int64, error)
	GetActiveGroupCountByDay(ctx context.Context, day time.Time) (int64, error)
	ListActiveUserIDsByDay(ctx context.Context, day time.Time, page, pageSize int32) ([]string, int64, error)
	ListActiveGroupsByDay(ctx context.Context, day time.Time, page, pageSize int32) ([]*biz.ActiveGroupItem, int64, error)
	GetTotalUserCount(ctx context.Context) (int64, error)
	GetTotalGroupCount(ctx context.Context) (int64, error)
	GetTodayNewUserCount(ctx context.Context) (int64, error)
	EnsureActiveStatIndexes(ctx context.Context) error
}

type groupInfoDAO struct {
	mysql       *mysql.MysqlConn // dtalk 数据库连接
	mysqlRecord *mysql.MysqlConn // dtalk_record 数据库连接
}

// NewGroupInfoDAO 创建群组信息DAO
func NewGroupInfoDAO(mysql *mysql.MysqlConn, mysqlRecord *mysql.MysqlConn) GroupInfoDAO {
	return &groupInfoDAO{
		mysql:       mysql,
		mysqlRecord: mysqlRecord,
	}
}

// GetGroupInfo 获取群组信息
func (d *groupInfoDAO) GetGroupInfo(ctx context.Context, groupId *big.Int) (*biz.GroupInfo, error) {
	records, err := d.mysql.Query(_GetGroupInfoByGroupId, groupId.String())
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("group not found")
	}

	record := records[0]
	groupInfo := d.convertRecordToBiz(record)
	return groupInfo, nil
}

// GetGroupList 获取群组列表
func (d *groupInfoDAO) GetGroupList(ctx context.Context, page, pageSize int32, keyword string, status int32) ([]*biz.GroupInfo, int64, error) {
	var conditions []string
	var args []interface{}

	// 构建查询条件
	if keyword != "" {
		conditions = append(conditions, "(group_name LIKE ? OR group_mark_id LIKE ? OR group_id LIKE ?)")
		args = append(args, "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if status >= 0 {
		conditions = append(conditions, "group_status = ?")
		args = append(args, status)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	// 查询总数
	countSQL := _GetGroupInfoList + whereClause
	countRecords, err := d.mysql.Query(countSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	total := int64(len(countRecords))

	// 查询列表
	offset := (page - 1) * pageSize
	listSQL := _GetGroupInfoList + whereClause + " ORDER BY group_create_time DESC LIMIT ? OFFSET ?"
	listArgs := append(args, pageSize, offset)

	records, err := d.mysql.Query(listSQL, listArgs...)
	if err != nil {
		return nil, 0, err
	}

	var groups []*biz.GroupInfo
	for _, record := range records {
		groupInfo := d.convertRecordToBiz(record)
		groups = append(groups, groupInfo)
	}

	return groups, total, nil
}

// GetGroupsByOwner 根据创建者地址获取群组列表
func (d *groupInfoDAO) GetGroupsByOwner(ctx context.Context, ownerId string, page, pageSize int32) ([]*biz.GroupInfo, int64, error) {
	// 查询总数
	countRecords, err := d.mysql.Query(_GetGroupCountByOwner, ownerId)
	if err != nil {
		return nil, 0, err
	}
	total := int64(0)
	if len(countRecords) > 0 {
		if count, ok := countRecords[0]["count"]; ok {
			total, _ = strconv.ParseInt(count, 10, 64)
		}
	}

	// 查询列表
	offset := (page - 1) * pageSize
	listSQL := _GetGroupsByOwner + " ORDER BY group_create_time DESC LIMIT ? OFFSET ?"
	records, err := d.mysql.Query(listSQL, ownerId, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}

	var groups []*biz.GroupInfo
	for _, record := range records {
		groupInfo := d.convertRecordToBiz(record)
		groups = append(groups, groupInfo)
	}

	return groups, total, nil
}

// GetGroupCountByOwner 根据创建者地址获取群组数量
func (d *groupInfoDAO) GetGroupCountByOwner(ctx context.Context, ownerId string) (int64, error) {
	records, err := d.mysql.Query(_GetGroupCountByOwner, ownerId)
	if err != nil {
		return 0, err
	}
	if len(records) == 0 {
		return 0, nil
	}
	if count, ok := records[0]["count"]; ok {
		return strconv.ParseInt(count, 10, 64)
	}
	return 0, nil
}

// UpdateGroupMaximum 更新群组最大人数
func (d *groupInfoDAO) UpdateGroupMaximum(ctx context.Context, groupId *big.Int, maximum int32) error {
	_, _, err := d.mysql.Exec(_UpdateGroupMaximum, maximum, time.Now().UnixMilli(), groupId.String())
	return err
}

// UpdateGroupStatus 更新群组状态
func (d *groupInfoDAO) UpdateGroupStatus(ctx context.Context, groupId *big.Int, status int32) error {
	_, _, err := d.mysql.Exec(_UpdateGroupStatus, status, time.Now().UnixMilli(), groupId.String())
	return err
}

// UpdateGroupName 更新群组名称
func (d *groupInfoDAO) UpdateGroupName(ctx context.Context, groupId *big.Int, name string) error {
	_, _, err := d.mysql.Exec(_UpdateGroupName, name, time.Now().UnixMilli(), groupId.String())
	return err
}

// GetGroupStats 获取群组统计信息
func (d *groupInfoDAO) GetGroupStats(ctx context.Context) (*biz.GroupStats, error) {
	records, err := d.mysql.Query(_GetGroupStats)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return &biz.GroupStats{}, nil
	}

	record := records[0]
	stats := &biz.GroupStats{}

	if total, ok := record["total"]; ok {
		stats.TotalGroups, _ = strconv.ParseInt(total, 10, 64)
	}
	if active, ok := record["active"]; ok {
		stats.ActiveGroups, _ = strconv.ParseInt(active, 10, 64)
	}
	if banned, ok := record["banned"]; ok {
		stats.BannedGroups, _ = strconv.ParseInt(banned, 10, 64)
	}
	if dissolved, ok := record["dissolved"]; ok {
		stats.DissolvedGroups, _ = strconv.ParseInt(dissolved, 10, 64)
	}

	return stats, nil
}

// convertRecordToBiz 转换记录为业务模型
func (d *groupInfoDAO) convertRecordToBiz(record map[string]string) *biz.GroupInfo {
	groupInfo := &biz.GroupInfo{}

	if groupId, ok := record["group_id"]; ok {
		//fmt.Printf("groupId++++++++++++++++++:%s\n", groupId)
		groupInfo.GroupId = new(big.Int)
		groupInfo.GroupId.SetString(groupId, 10)
		//fmt.Printf("groupInfo.GroupId:%s\n", groupInfo.GroupId)
	}
	if groupMarkId, ok := record["group_mark_id"]; ok {
		groupInfo.GroupMarkId = groupMarkId
	}
	if groupName, ok := record["group_name"]; ok {
		// 尝试解密群组名称
		if aesKey, hasKey := record["group_aes_key"]; hasKey {
			decryptedName := d.decryptGroupName(groupName, aesKey)
			groupInfo.GroupName = decryptedName
		} else {
			groupInfo.GroupName = groupName
		}
	}
	if groupAvatar, ok := record["group_avatar"]; ok {
		groupInfo.GroupAvatar = groupAvatar
	}
	if groupMemberNum, ok := record["group_member_num"]; ok {
		if num, err := strconv.ParseInt(groupMemberNum, 10, 32); err == nil {
			groupInfo.GroupMemberNum = int32(num)
		}
	}
	if groupMaximum, ok := record["group_maximum"]; ok {
		if max, err := strconv.ParseInt(groupMaximum, 10, 32); err == nil {
			groupInfo.GroupMaximum = int32(max)
		}
	}
	if groupIntroduce, ok := record["group_introduce"]; ok {
		groupInfo.GroupIntroduce = groupIntroduce
	}
	if groupStatus, ok := record["group_status"]; ok {
		if status, err := strconv.ParseInt(groupStatus, 10, 32); err == nil {
			groupInfo.GroupStatus = int32(status)
		}
	}
	if groupOwnerId, ok := record["group_owner_id"]; ok {
		groupInfo.GroupOwnerId = groupOwnerId
	}
	if groupCreateTime, ok := record["group_create_time"]; ok {
		groupInfo.GroupCreateTime, _ = strconv.ParseInt(groupCreateTime, 10, 64)
	}
	if groupUpdateTime, ok := record["group_update_time"]; ok {
		groupInfo.GroupUpdateTime, _ = strconv.ParseInt(groupUpdateTime, 10, 64)
	}
	if groupJoinType, ok := record["group_join_type"]; ok {
		if joinType, err := strconv.ParseInt(groupJoinType, 10, 32); err == nil {
			groupInfo.GroupJoinType = int32(joinType)
		}
	}
	if groupMuteType, ok := record["group_mute_type"]; ok {
		if muteType, err := strconv.ParseInt(groupMuteType, 10, 32); err == nil {
			groupInfo.GroupMuteType = int32(muteType)
		}
	}
	if groupFriendType, ok := record["group_friend_type"]; ok {
		if friendType, err := strconv.ParseInt(groupFriendType, 10, 32); err == nil {
			groupInfo.GroupFriendType = int32(friendType)
		}
	}
	if groupAESKey, ok := record["group_aes_key"]; ok {
		groupInfo.GroupAESKey = groupAESKey
	}
	if groupPubName, ok := record["group_pub_name"]; ok {
		groupInfo.GroupPubName = groupPubName
	}
	if groupType, ok := record["group_type"]; ok {
		if gType, err := strconv.ParseInt(groupType, 10, 32); err == nil {
			groupInfo.GroupType = int32(gType)
		}
	}

	return groupInfo
}

// decryptGroupName 解密群组名称
func (d *groupInfoDAO) decryptGroupName(encryptedName, aesKey string) string {
	if encryptedName == "" || aesKey == "" {
		return encryptedName
	}

	// 解码Base64
	encryptedBytes, err := base64.StdEncoding.DecodeString(encryptedName)
	if err != nil {
		// 如果解码失败，返回原始字符串
		return encryptedName
	}

	// 解码AES密钥（十六进制格式）
	keyBytes, err := hex.DecodeString(aesKey)
	if err != nil {
		// 如果密钥解码失败，返回原始字符串
		return encryptedName
	}

	// 创建AES cipher
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		// 如果创建cipher失败，返回原始字符串
		return encryptedName
	}

	// 使用AES解密
	if len(encryptedBytes) < aes.BlockSize {
		// 数据太短，无法解密
		return encryptedName
	}

	// 使用CBC模式解密
	iv := encryptedBytes[:aes.BlockSize]
	ciphertext := encryptedBytes[aes.BlockSize:]

	mode := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(ciphertext))
	mode.CryptBlocks(decrypted, ciphertext)

	// 去除PKCS7填充
	decrypted = d.removePKCS7Padding(decrypted)

	return string(decrypted)
}

// removePKCS7Padding 移除PKCS7填充
func (d *groupInfoDAO) removePKCS7Padding(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	padding := int(data[len(data)-1])
	if padding > len(data) || padding == 0 {
		return data
	}

	// 检查填充是否正确
	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return data
		}
	}

	return data[:len(data)-padding]
}

// GetGroupMembers 分页获取群成员。page 从 1 开始。
func (d *groupInfoDAO) GetGroupMembers(ctx context.Context, groupId *big.Int, page, pageSize int32) ([]*biz.GroupMember, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	groupID := groupId.String()
	countRecords, err := d.mysql.Query(_CountMembersByGroupId, groupID)
	if err != nil {
		return nil, 0, fmt.Errorf("count group members groupId=%s: %w", groupID, err)
	}
	total := int64(0)
	if len(countRecords) > 0 {
		total, _ = strconv.ParseInt(countRecords[0]["count"], 10, 64)
	}

	offset := (page - 1) * pageSize
	records, err := d.mysql.Query(_ListMembersByGroupId, groupID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list group members groupId=%s page=%d pageSize=%d: %w", groupID, page, pageSize, err)
	}

	members := make([]*biz.GroupMember, 0, len(records))
	for _, record := range records {
		member := &biz.GroupMember{
			GroupId:             groupId,
			GroupMemberId:       record["group_member_id"],
			GroupMemberName:     record["group_member_name"],
			GroupMemberJoinTime: parseInt64(record["group_member_join_time"]),
		}

		// 解析成员类型
		if memberType, ok := record["group_member_type"]; ok {
			if t, err := strconv.ParseInt(memberType, 10, 32); err == nil {
				member.GroupMemberType = int32(t)
			}
		}

		// 解析禁言时间
		if muteTime, ok := record["group_member_mute_time"]; ok {
			member.GroupMemberMuteTime = parseInt64(muteTime)
		}

		members = append(members, member)
	}

	return members, total, nil
}

// GetTodayGroupCount 获取今日创建的群聊数量
func (d *groupInfoDAO) GetTodayGroupCount(ctx context.Context) (int64, error) {
	now := time.Now()
	// 获取今天的开始时间（00:00:00）
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	// 获取明天的开始时间（作为今天的结束时间）
	tomorrowStart := todayStart.AddDate(0, 0, 1)

	todayStartUnix := todayStart.UnixMilli()
	tomorrowStartUnix := tomorrowStart.UnixMilli()

	records, err := d.mysql.Query(_GetTodayGroupCount, todayStartUnix, tomorrowStartUnix)
	if err != nil {
		return 0, err
	}
	if len(records) == 0 {
		return 0, nil
	}
	if count, ok := records[0]["count"]; ok {
		return strconv.ParseInt(count, 10, 64)
	}
	return 0, nil
}

// GetTodayActiveUserCount 获取今日活跃用户数（今日在 dtalk_signal_content 有记录的去重 uid）
func (d *groupInfoDAO) GetTodayActiveUserCount(ctx context.Context) (int64, error) {
	return d.GetActiveUserCountByDay(ctx, time.Now())
}

// GetActiveUserCountByDay 获取指定自然日的活跃用户数
func (d *groupInfoDAO) GetActiveUserCountByDay(ctx context.Context, day time.Time) (int64, error) {
	if d.mysqlRecord == nil {
		return 0, fmt.Errorf("dtalk_record database connection not initialized")
	}
	start, end := dayBounds(day)
	records, err := d.mysqlRecord.Query(_GetTodayActiveUserCount, start.UnixMilli(), end.UnixMilli())
	if err != nil {
		fmt.Printf("[GroupInfoDAO] GetActiveUserCountByDay failed: day=%s err=%v\n", day.Format("2006-01-02"), err)
		return 0, err
	}
	if len(records) == 0 {
		return 0, nil
	}
	if count, ok := records[0]["count"]; ok {
		return strconv.ParseInt(count, 10, 64)
	}
	return 0, nil
}

// GetTodayActiveGroupCount 获取今日有消息的群数量。数据在 dtalk_group_msg_relation。
func (d *groupInfoDAO) GetTodayActiveGroupCount(ctx context.Context) (int64, error) {
	return d.GetActiveGroupCountByDay(ctx, time.Now())
}

// GetActiveGroupCountByDay 获取指定自然日的活跃群组数
func (d *groupInfoDAO) GetActiveGroupCountByDay(ctx context.Context, day time.Time) (int64, error) {
	if d.mysqlRecord == nil {
		return 0, fmt.Errorf("dtalk_record database connection not initialized")
	}
	start, end := dayBounds(day)
	records, err := d.mysqlRecord.Query(_GetTodayActiveGroupCount, start.UnixMilli(), end.UnixMilli())
	if err != nil {
		fmt.Printf("[GroupInfoDAO] GetActiveGroupCountByDay failed: day=%s err=%v\n", day.Format("2006-01-02"), err)
		return 0, err
	}
	if len(records) == 0 {
		return 0, nil
	}
	if count, ok := records[0]["count"]; ok {
		return strconv.ParseInt(count, 10, 64)
	}
	return 0, nil
}

// ListActiveUserIDsByDay 分页返回指定自然日的活跃地址。总数与计数接口同一条件。
func (d *groupInfoDAO) ListActiveUserIDsByDay(ctx context.Context, day time.Time, page, pageSize int32) ([]string, int64, error) {
	if d.mysqlRecord == nil {
		return nil, 0, fmt.Errorf("dtalk_record database connection not initialized")
	}
	page, pageSize = clampActiveDetailPage(page, pageSize)
	total, err := d.GetActiveUserCountByDay(ctx, day)
	if err != nil {
		return nil, 0, err
	}
	start, end := dayBounds(day)
	offset := (page - 1) * pageSize
	records, err := d.mysqlRecord.Query(_ListActiveUserIDs, start.UnixMilli(), end.UnixMilli(), pageSize, offset)
	if err != nil {
		fmt.Printf("[GroupInfoDAO] ListActiveUserIDsByDay failed: day=%s page=%d pageSize=%d err=%v\n",
			day.Format("2006-01-02"), page, pageSize, err)
		return nil, 0, err
	}
	list := make([]string, 0, len(records))
	for _, record := range records {
		if uid := strings.TrimSpace(record["uid"]); uid != "" {
			list = append(list, uid)
		}
	}
	return list, total, nil
}

// ListActiveGroupsByDay 分页返回指定自然日有消息的群，并带上解密后的群名。
func (d *groupInfoDAO) ListActiveGroupsByDay(ctx context.Context, day time.Time, page, pageSize int32) ([]*biz.ActiveGroupItem, int64, error) {
	if d.mysqlRecord == nil {
		return nil, 0, fmt.Errorf("dtalk_record database connection not initialized")
	}
	page, pageSize = clampActiveDetailPage(page, pageSize)
	total, err := d.GetActiveGroupCountByDay(ctx, day)
	if err != nil {
		return nil, 0, err
	}
	start, end := dayBounds(day)
	offset := (page - 1) * pageSize
	records, err := d.mysqlRecord.Query(_ListActiveGroupIDs, start.UnixMilli(), end.UnixMilli(), pageSize, offset)
	if err != nil {
		fmt.Printf("[GroupInfoDAO] ListActiveGroupsByDay failed: day=%s page=%d pageSize=%d err=%v\n",
			day.Format("2006-01-02"), page, pageSize, err)
		return nil, 0, err
	}
	ids := make([]string, 0, len(records))
	for _, record := range records {
		if id := strings.TrimSpace(record["group_id"]); id != "" {
			ids = append(ids, id)
		}
	}
	names, err := d.groupNamesByIDs(ids)
	if err != nil {
		fmt.Printf("[GroupInfoDAO] groupNamesByIDs failed: day=%s idCount=%d err=%v\n",
			day.Format("2006-01-02"), len(ids), err)
		return nil, 0, err
	}
	list := make([]*biz.ActiveGroupItem, 0, len(ids))
	for _, id := range ids {
		list = append(list, &biz.ActiveGroupItem{GroupID: id, GroupName: names[id]})
	}
	return list, total, nil
}

func (d *groupInfoDAO) groupNamesByIDs(ids []string) (map[string]string, error) {
	names := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return names, nil
	}
	if d.mysql == nil {
		return nil, fmt.Errorf("dtalk database connection not initialized")
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	query := "SELECT group_id, group_name, group_aes_key FROM dtalk_group_info WHERE group_id IN (" + placeholders + ")"
	records, err := d.mysql.Query(query, args...)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		id := strings.TrimSpace(record["group_id"])
		if id == "" {
			continue
		}
		names[id] = d.decryptGroupName(record["group_name"], record["group_aes_key"])
	}
	return names, nil
}

func clampActiveDetailPage(page, pageSize int32) (int32, int32) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func dayBounds(day time.Time) (time.Time, time.Time) {
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	return start, start.AddDate(0, 0, 1)
}

// GetTotalUserCount 获取聊天系统总用户数
func (d *groupInfoDAO) GetTotalUserCount(ctx context.Context) (int64, error) {
	// 使用 dtalk_record 数据库连接查询
	if d.mysqlRecord == nil {
		return 0, fmt.Errorf("dtalk_record database connection not initialized")
	}
	records, err := d.mysqlRecord.Query(_GetTotalUserCount)
	if err != nil {
		return 0, err
	}
	if len(records) == 0 {
		return 0, nil
	}
	if count, ok := records[0]["count"]; ok {
		return strconv.ParseInt(count, 10, 64)
	}
	return 0, nil
}

// GetTotalGroupCount 群组总数。快照入库用，不再连带统计今日活跃。
func (d *groupInfoDAO) GetTotalGroupCount(ctx context.Context) (int64, error) {
	records, err := d.mysql.Query(_GetTotalGroupCount)
	if err != nil {
		fmt.Printf("[GroupInfoDAO] GetTotalGroupCount failed: err=%v\n", err)
		return 0, err
	}
	if len(records) == 0 {
		return 0, nil
	}
	count, _ := strconv.ParseInt(records[0]["count"], 10, 64)
	return count, nil
}

// EnsureActiveStatIndexes 给活跃统计补上按天可检索的索引。已存在则跳过。
// 建索引用 INPLACE、不加锁，避免堵住消息写入；第一次会扫大表，只在后台跑。
func (d *groupInfoDAO) EnsureActiveStatIndexes(ctx context.Context) error {
	if d.mysqlRecord == nil {
		return fmt.Errorf("dtalk_record database connection not initialized")
	}
	indexes := []struct {
		table string
		name  string
		ddl   string
	}{
		{
			table: "dtalk_signal_content",
			name:  "idx_signal_create_uid",
			ddl:   "ALTER TABLE dtalk_signal_content ADD INDEX idx_signal_create_uid (create_time, uid), ALGORITHM=INPLACE, LOCK=NONE",
		},
		{
			table: "dtalk_group_msg_relation",
			name:  "idx_group_msg_type_time_other",
			ddl:   "ALTER TABLE dtalk_group_msg_relation ADD INDEX idx_group_msg_type_time_other (type, create_time, other_uid), ALGORITHM=INPLACE, LOCK=NONE",
		},
	}
	for _, idx := range indexes {
		exists, err := d.recordIndexExists(idx.table, idx.name)
		if err != nil {
			fmt.Printf("[GroupInfoDAO] EnsureActiveStatIndexes check failed: table=%s index=%s err=%v\n", idx.table, idx.name, err)
			return err
		}
		if exists {
			continue
		}
		started := time.Now()
		fmt.Printf("[GroupInfoDAO] EnsureActiveStatIndexes start: table=%s index=%s\n", idx.table, idx.name)
		if _, _, err = d.mysqlRecord.Exec(idx.ddl); err != nil {
			fmt.Printf("[GroupInfoDAO] EnsureActiveStatIndexes failed: table=%s index=%s elapsed=%s err=%v\n",
				idx.table, idx.name, time.Since(started), err)
			return err
		}
		fmt.Printf("[GroupInfoDAO] EnsureActiveStatIndexes ok: table=%s index=%s elapsed=%s\n",
			idx.table, idx.name, time.Since(started))
	}
	return nil
}

func (d *groupInfoDAO) recordIndexExists(table, name string) (bool, error) {
	records, err := d.mysqlRecord.Query(
		`SELECT COUNT(*) AS count FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?`,
		table, name,
	)
	if err != nil {
		return false, err
	}
	if len(records) == 0 {
		return false, nil
	}
	n, _ := strconv.ParseInt(records[0]["count"], 10, 64)
	return n > 0, nil
}

// GetTodayNewUserCount 获取今日新增用户数
// 注意：这里统计的是今日首次接收信号的用户数（基于 dtalk_signal_content 表）
func (d *groupInfoDAO) GetTodayNewUserCount(ctx context.Context) (int64, error) {
	// 使用 dtalk_record 数据库连接查询
	if d.mysqlRecord == nil {
		return 0, fmt.Errorf("dtalk_record database connection not initialized")
	}

	now := time.Now()
	// 获取今天的开始时间（00:00:00）
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	// 获取明天的开始时间（作为今天的结束时间）
	tomorrowStart := todayStart.AddDate(0, 0, 1)

	todayStartUnix := todayStart.UnixMilli()
	tomorrowStartUnix := tomorrowStart.UnixMilli()

	// 查询今日首次接收信号的用户（即今日新增用户）
	// 使用子查询：今日有记录的用户，且该用户的历史最早记录也在今日
	query := `SELECT COUNT(DISTINCT uid) as count FROM dtalk_signal_content 
		WHERE create_time >= ? AND create_time < ? 
		AND uid NOT IN (
			SELECT DISTINCT uid FROM dtalk_signal_content 
			WHERE create_time < ?
		)`

	records, err := d.mysqlRecord.Query(query, todayStartUnix, tomorrowStartUnix, todayStartUnix)
	if err != nil {
		return 0, err
	}
	if len(records) == 0 {
		return 0, nil
	}
	if count, ok := records[0]["count"]; ok {
		return strconv.ParseInt(count, 10, 64)
	}
	return 0, nil
}

// parseInt64 解析字符串为 int64
func parseInt64(s string) int64 {
	if s == "" {
		return 0
	}
	val, _ := strconv.ParseInt(s, 10, 64)
	return val
}
