package types

// APIResponse 统一API响应结构
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// PageInfo 分页信息
type PageInfo struct {
	Page     int32 `json:"page"`
	PageSize int32 `json:"pageSize"`
	Total    int64 `json:"total"`
}

// GroupStats 群组统计信息
type GroupStats struct {
	TotalGroups     int64 `json:"totalGroups"`
	ActiveGroups    int64 `json:"activeGroups"`
	BannedGroups    int64 `json:"bannedGroups"`
	DissolvedGroups int64 `json:"dissolvedGroups"`
}

// WhitelistStats 白名单统计信息
type WhitelistStats struct {
	TotalWhitelist    int64 `json:"totalWhitelist"`
	ActiveWhitelist   int64 `json:"activeWhitelist"`
	DisabledWhitelist int64 `json:"disabledWhitelist"`
}
