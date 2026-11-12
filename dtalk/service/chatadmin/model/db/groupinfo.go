package db

import (
	"time"
)

// GroupInfo 群组信息数据库模型
type GroupInfo struct {
	GroupId         int64  `json:"groupId" db:"group_id"`
	GroupMarkId     string `json:"groupMarkId" db:"group_mark_id"`
	GroupName       string `json:"groupName" db:"group_name"`
	GroupAvatar     string `json:"groupAvatar" db:"group_avatar"`
	GroupMemberNum  int32  `json:"groupMemberNum" db:"group_member_num"`
	GroupMaximum    int32  `json:"groupMaximum" db:"group_maximum"`
	GroupIntroduce  string `json:"groupIntroduce" db:"group_introduce"`
	GroupStatus     int32  `json:"groupStatus" db:"group_status"`
	GroupOwnerId    string `json:"groupOwnerId" db:"group_owner_id"`
	GroupCreateTime int64  `json:"groupCreateTime" db:"group_create_time"`
	GroupUpdateTime int64  `json:"groupUpdateTime" db:"group_update_time"`
	GroupJoinType   int32  `json:"groupJoinType" db:"group_join_type"`
	GroupMuteType   int32  `json:"groupMuteType" db:"group_mute_type"`
	GroupFriendType int32  `json:"groupFriendType" db:"group_friend_type"`
	GroupAESKey     string `json:"groupAESKey" db:"group_aes_key"`
	GroupPubName    string `json:"groupPubName" db:"group_pub_name"`
	GroupType       int32  `json:"groupType" db:"group_type"`
}

// WhitelistInfo 白名单信息数据库模型
type WhitelistInfo struct {
	Id         int64     `json:"id" db:"id"`
	Address    string    `json:"address" db:"address"`
	Remark     string    `json:"remark" db:"remark"`
	Status     int32     `json:"status" db:"status"`
	MaxMembers int32     `json:"maxMembers" db:"max_members"`
	CreatedBy  string    `json:"createdBy" db:"created_by"`
	CreatedAt  time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt  time.Time `json:"updatedAt" db:"updated_at"`
}
