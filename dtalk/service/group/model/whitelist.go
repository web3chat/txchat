package model

// CheckWhitelistResp 白名单检查响应
type CheckWhitelistResp struct {
	IsInWhitelist bool   `json:"is_in_whitelist"` // 是否在白名单中
	MaxMembers    int32  `json:"max_members"`     // 最大群成员数量限制
	MaxGroups     int32  `json:"max_groups"`      // 最大创建群聊个数限制
	Address       string `json:"address"`         // 检查的地址
}

// WhitelistInfo 白名单信息
type WhitelistInfo struct {
	ID         int64  `json:"id" db:"id"`
	Address    string `json:"address" db:"address"`
	Remark     string `json:"remark" db:"remark"`
	Status     int32  `json:"status" db:"status"`           // 状态：1=启用，0=禁用
	MaxMembers int32  `json:"max_members" db:"max_members"` // 群聊最大人数上限
	CreatedBy  string `json:"created_by" db:"created_by"`   // 创建人地址
	CreatedAt  int64  `json:"created_at" db:"created_at"`   // 创建时间
	UpdatedAt  int64  `json:"updated_at" db:"updated_at"`   // 更新时间
}
