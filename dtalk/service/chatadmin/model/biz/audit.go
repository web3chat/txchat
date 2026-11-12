package biz

// AdminAudit 运营台操作留痕。
type AdminAudit struct {
	ID         int64  `json:"id"`
	Operator   string `json:"operator"`
	Action     string `json:"action"`
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
	Extra      string `json:"extra"`
	ClientIP   string `json:"clientIp"`
	CreatedAt  int64  `json:"createdAt"`
}

// AdminAuditList 操作留痕分页结果。
type AdminAuditList struct {
	List     []*AdminAudit `json:"list"`
	Total    int64         `json:"total"`
	Page     int32         `json:"page"`
	PageSize int32         `json:"pageSize"`
}
