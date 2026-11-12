package biz

import (
	"encoding/json"
	"math/big"
	"time"
)

// GroupInfo 群组信息业务模型
type GroupInfo struct {
	GroupId         *big.Int `json:"groupId"`
	GroupMarkId     string   `json:"groupMarkId"`
	GroupName       string   `json:"groupName"`
	GroupAvatar     string   `json:"groupAvatar"`
	GroupMemberNum  int32    `json:"groupMemberNum"`
	GroupMaximum    int32    `json:"groupMaximum"`
	GroupIntroduce  string   `json:"groupIntroduce"`
	GroupStatus     int32    `json:"groupStatus"`
	GroupOwnerId    string   `json:"groupOwnerId"`
	GroupCreateTime int64    `json:"groupCreateTime"`
	GroupUpdateTime int64    `json:"groupUpdateTime"`
	GroupJoinType   int32    `json:"groupJoinType"`
	GroupMuteType   int32    `json:"groupMuteType"`
	GroupFriendType int32    `json:"groupFriendType"`
	GroupAESKey     string   `json:"groupAESKey"`
	GroupPubName    string   `json:"groupPubName"`
	GroupType       int32    `json:"groupType"`
}

// MarshalJSON 自定义JSON序列化，将big.Int序列化为字符串
func (g *GroupInfo) MarshalJSON() ([]byte, error) {
	type Alias GroupInfo
	return json.Marshal(&struct {
		GroupId string `json:"groupId"`
		*Alias
	}{
		GroupId: g.GroupId.String(),
		Alias:   (*Alias)(g),
	})
}

// WhitelistInfo 白名单信息业务模型
type WhitelistInfo struct {
	Id         *big.Int  `json:"id"`
	Address    string    `json:"address"`
	Remark     string    `json:"remark"`
	Status     int32     `json:"status"`
	MaxMembers int32     `json:"maxMembers"`
	MaxGroups  int32     `json:"maxGroups"` // 最大创建群聊个数
	CreatedBy  string    `json:"createdBy"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// MarshalJSON 自定义JSON序列化，将big.Int序列化为字符串
func (w *WhitelistInfo) MarshalJSON() ([]byte, error) {
	type Alias WhitelistInfo
	return json.Marshal(&struct {
		Id string `json:"id"`
		*Alias
	}{
		Id:    w.Id.String(),
		Alias: (*Alias)(w),
	})
}

// GroupListRequest 群组列表请求
type GroupListRequest struct {
	Page     int32  `json:"page" form:"page" binding:"min=1"`
	PageSize int32  `json:"pageSize" form:"pageSize" binding:"min=1,max=100"`
	Keyword  string `json:"keyword" form:"keyword"`
	Status   int32  `json:"status" form:"status"`
}

// GroupListResponse 群组列表响应
type GroupListResponse struct {
	Total int64       `json:"total"`
	List  []GroupInfo `json:"list"`
}

// UpdateGroupRequest 更新群组请求
type UpdateGroupRequest struct {
	GroupId      *big.Int `json:"groupId" binding:"required"`
	GroupMaximum int32    `json:"groupMaximum,omitempty"`
	GroupStatus  int32    `json:"groupStatus,omitempty"`
	GroupName    string   `json:"groupName,omitempty"`
}

// UnmarshalJSON 自定义JSON反序列化，将字符串ID解析为big.Int
func (u *UpdateGroupRequest) UnmarshalJSON(data []byte) error {
	type Alias UpdateGroupRequest
	aux := &struct {
		GroupId string `json:"groupId"`
		*Alias
	}{
		Alias: (*Alias)(u),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.GroupId != "" {
		u.GroupId = new(big.Int)
		_, ok := u.GroupId.SetString(aux.GroupId, 10)
		if !ok {
			return json.Unmarshal(data, u)
		}
	}

	return nil
}

// WhitelistListRequest 白名单列表请求
type WhitelistListRequest struct {
	Page     int32  `json:"page" form:"page" binding:"min=1"`
	PageSize int32  `json:"pageSize" form:"pageSize" binding:"min=1,max=100"`
	Keyword  string `json:"keyword" form:"keyword"`
	Status   int32  `json:"status" form:"status"`
}

// WhitelistListResponse 白名单列表响应
type WhitelistListResponse struct {
	Total int64           `json:"total"`
	List  []WhitelistInfo `json:"list"`
}

// AddWhitelistRequest 添加白名单请求
type AddWhitelistRequest struct {
	Address    string `json:"address" binding:"required"`
	Remark     string `json:"remark"`
	MaxMembers int32  `json:"maxMembers" binding:"min=1"`
	MaxGroups  int32  `json:"maxGroups" binding:"min=1"` // 最大创建群聊个数
	CreatedBy  string `json:"createdBy" binding:"required"`
}

// UpdateWhitelistRequest 更新白名单请求
type UpdateWhitelistRequest struct {
	Id         *big.Int `json:"id" binding:"required"`
	Remark     string   `json:"remark"`
	Status     int32    `json:"status"`
	MaxMembers int32    `json:"maxMembers" binding:"min=1"`
	MaxGroups  int32    `json:"maxGroups" binding:"min=1"` // 最大创建群聊个数
}

// UnmarshalJSON 自定义JSON反序列化，将字符串ID解析为big.Int
func (u *UpdateWhitelistRequest) UnmarshalJSON(data []byte) error {
	type Alias UpdateWhitelistRequest
	aux := &struct {
		Id string `json:"id"`
		*Alias
	}{
		Alias: (*Alias)(u),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.Id != "" {
		u.Id = new(big.Int)
		_, ok := u.Id.SetString(aux.Id, 10)
		if !ok {
			return json.Unmarshal(data, u)
		}
	}

	return nil
}

// DeleteWhitelistRequest 删除白名单请求
type DeleteWhitelistRequest struct {
	Id *big.Int `json:"id" binding:"required"`
}

// UnmarshalJSON 自定义JSON反序列化，将字符串ID解析为big.Int
func (d *DeleteWhitelistRequest) UnmarshalJSON(data []byte) error {
	type Alias DeleteWhitelistRequest
	aux := &struct {
		Id string `json:"id"`
		*Alias
	}{
		Alias: (*Alias)(d),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.Id != "" {
		d.Id = new(big.Int)
		_, ok := d.Id.SetString(aux.Id, 10)
		if !ok {
			return json.Unmarshal(data, d)
		}
	}

	return nil
}

// GroupStats 群组统计信息
type GroupStats struct {
	TotalGroups       int64 `json:"totalGroups"`
	ActiveGroups      int64 `json:"activeGroups"`
	BannedGroups      int64 `json:"bannedGroups"`
	DissolvedGroups   int64 `json:"dissolvedGroups"`
	TodayActiveUsers  int64 `json:"todayActiveUsers"`
	TodayActiveGroups int64 `json:"todayActiveGroups"`
}

// WhitelistStats 白名单统计信息
type WhitelistStats struct {
	TotalWhitelist    int64 `json:"totalWhitelist"`
	ActiveWhitelist   int64 `json:"activeWhitelist"`
	DisabledWhitelist int64 `json:"disabledWhitelist"`
}

// GroupMember 群成员信息业务模型
type GroupMember struct {
	GroupId             *big.Int `json:"groupId"`
	GroupMemberId       string   `json:"groupMemberId"`
	GroupMemberName     string   `json:"groupMemberName"`
	GroupMemberType     int32    `json:"groupMemberType"`     // 用户角色，0=群员,1=管理员, 2=群主，10=退群
	GroupMemberMuteTime int64    `json:"groupMemberMuteTime"` // 该用户被禁言结束的时间，9223372036854775807=永久禁言
	GroupMemberJoinTime int64    `json:"groupMemberJoinTime"`
}

// MarshalJSON 自定义JSON序列化，将big.Int序列化为字符串
func (g *GroupMember) MarshalJSON() ([]byte, error) {
	type Alias GroupMember
	return json.Marshal(&struct {
		GroupId string `json:"groupId"`
		*Alias
	}{
		GroupId: g.GroupId.String(),
		Alias:   (*Alias)(g),
	})
}
