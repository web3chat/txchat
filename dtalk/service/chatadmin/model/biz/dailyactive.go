package biz

// DailyActiveRecord 每日活跃入库记录
type DailyActiveRecord struct {
	StatDate     string `json:"statDate"` // YYYY-MM-DD
	ActiveUsers  int64  `json:"activeUsers"`
	ActiveGroups int64  `json:"activeGroups"`
	TotalUsers   int64  `json:"totalUsers"`
	TotalGroups  int64  `json:"totalGroups"`
	UpdatedAt    int64  `json:"updatedAt"`
}

// DailyActiveListRequest 每日活跃列表查询（支持日期范围与多 date 并行筛选）
type DailyActiveListRequest struct {
	StartDate string   // YYYY-MM-DD，含
	EndDate   string   // YYYY-MM-DD，含
	Dates     []string // 多个精确日期：?date=a&date=b
	Page      int32
	PageSize  int32
}

// DailyActiveListResponse 每日活跃列表响应
type DailyActiveListResponse struct {
	List     []*DailyActiveRecord `json:"list"`
	Total    int64                `json:"total"`
	Page     int32                `json:"page"`
	PageSize int32                `json:"pageSize"`
}

// ActiveAddressPage 某一天的活跃地址分页
type ActiveAddressPage struct {
	Date     string   `json:"date"`
	List     []string `json:"list"`
	Total    int64    `json:"total"`
	Page     int32    `json:"page"`
	PageSize int32    `json:"pageSize"`
}

// ActiveGroupItem 某一天有消息的群
type ActiveGroupItem struct {
	GroupID   string `json:"groupId"`
	GroupName string `json:"groupName"`
}

// ActiveGroupPage 某一天的活跃群组分页
type ActiveGroupPage struct {
	Date     string             `json:"date"`
	List     []*ActiveGroupItem `json:"list"`
	Total    int64              `json:"total"`
	Page     int32              `json:"page"`
	PageSize int32              `json:"pageSize"`
}
