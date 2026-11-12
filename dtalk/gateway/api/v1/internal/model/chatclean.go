package model

// DateOption 手动删除时间范围选项
// 0=自定义, 1=近1天, 2=近3天, 3=近7天, 4=近30天, 5=近90天, 6=近180天, 7=近365天, 8=全部
type DateOption int32

const (
	DateOptionCustom      DateOption = 0
	DateOptionLast1Day    DateOption = 1
	DateOptionLast3Days   DateOption = 2
	DateOptionLast7Days   DateOption = 3
	DateOptionLast30Days  DateOption = 4
	DateOptionLast90Days  DateOption = 5
	DateOptionLast180Days DateOption = 6
	DateOptionLast365Days DateOption = 7
	DateOptionAllTime     DateOption = 8
)

// AutoDeleteOption 自动删除保留时长选项
// 0=不删除, 1=1天, 2=3天, 3=7天, 4=30天, 5=180天
type AutoDeleteOption int8

const (
	AutoDeleteNever   AutoDeleteOption = 0
	AutoDelete1Day    AutoDeleteOption = 1
	AutoDelete3Days   AutoDeleteOption = 2
	AutoDelete7Days   AutoDeleteOption = 3
	AutoDelete30Days  AutoDeleteOption = 4
	AutoDelete180Days AutoDeleteOption = 5
)

// CleanPrivateChatReq 手动删除私聊记录请求
type CleanPrivateChatReq struct {
	// 操作者 ID（从鉴权中间件中获取，不需要客户端填写）
	OperatorID string `json:"-"`
	// 对方用户 ID
	TargetID string `json:"targetId" binding:"required"`
	// 时间范围选项
	DateOpt DateOption `json:"dateOpt"`
	// 自定义起始时间（Unix ms），仅 dateOpt=0 时有效
	StartTime int64 `json:"startTime"`
	// 自定义结束时间（Unix ms），仅 dateOpt=0 时有效
	EndTime int64 `json:"endTime"`
}

// CleanPrivateChatResp 手动删除私聊记录响应
type CleanPrivateChatResp struct {
	// 实际删除的消息条数
	DeletedCount int64 `json:"deletedCount"`
}

// SetAutoDeleteConfigReq 设置自动删除配置请求
type SetAutoDeleteConfigReq struct {
	// 操作者 ID（从鉴权中间件中获取）
	OperatorID string `json:"-"`
	// 对方用户 ID
	TargetID string `json:"targetId" binding:"required"`
	// 自动删除选项
	Option AutoDeleteOption `json:"option"`
}

// AutoDeleteConfigResp 自动删除配置响应
type AutoDeleteConfigResp struct {
	// 当前选项
	Option AutoDeleteOption `json:"option"`
	// 最后修改人
	UpdatedBy string `json:"updatedBy"`
	// 最后修改时间（Unix ms）
	UpdatedAt int64 `json:"updatedAt"`
}

// GetAutoDeleteConfigReq 查询自动删除配置请求
type GetAutoDeleteConfigReq struct {
	// 操作者 ID（从鉴权中间件中获取）
	OperatorID string `json:"-"`
	// 对方用户 ID
	TargetID string `json:"targetId" binding:"required"`
}
