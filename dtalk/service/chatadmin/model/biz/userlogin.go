package biz

import "time"

// UserBinding 用户账号绑定信息（手机号、微信号、小红书号与聊天地址的绑定关系）
type UserBinding struct {
	Id            int64     `json:"id"`
	Phone         string    `json:"phone"`
	WechatId      string    `json:"wechatId"`
	XiaohongshuId string    `json:"xiaohongshuId"`
	ChatAddress   string    `json:"chatAddress"`
	IsDeleted     int32     `json:"isDeleted"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// LoginRecord 用户登录记录
type LoginRecord struct {
	Id         int64     `json:"id"`
	Uid        string    `json:"uid"`
	ClientIP   string    `json:"clientIp"`
	LoginAt    int64     `json:"loginAt"`
	DeviceType string    `json:"deviceType"`
	DeviceUUID string    `json:"deviceUuid"`
	DeviceName string    `json:"deviceName"`
	Country    string    `json:"country"`
	CityRegion string    `json:"cityRegion"`
	ISP        string    `json:"isp"`
	CreatedAt  time.Time `json:"createdAt"`
}

// UserLoginWithBinding 用户登录历史（含账号绑定信息）
type UserLoginWithBinding struct {
	Address  string         `json:"address"`
	Binding  *UserBinding   `json:"binding"`
	Logs     []*LoginRecord `json:"logs"`
	Total    int64          `json:"total"`
	Page     int32          `json:"page"`
	PageSize int32          `json:"pageSize"`
}

// UserLoginHistoryRequest 查询用户登录历史请求
type UserLoginHistoryRequest struct {
	Address   string `json:"address"`
	Page      int32  `json:"page"`
	PageSize  int32  `json:"pageSize"`
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
}

// AllLoginLogsRequest 全站登录日志分页查询（不按单用户强制过滤）
type AllLoginLogsRequest struct {
	Page      int32    `json:"page"`
	PageSize  int32    `json:"pageSize"`
	StartTime int64    `json:"startTime"`
	EndTime   int64    `json:"endTime"`
	Uid       []string `json:"uid"` // 可选；多地址并行筛选时为 OR（SQL IN），可与 ?uid=a&uid=b 形式对应
}

// AllLoginLogsResponse 全站登录日志列表
type AllLoginLogsResponse struct {
	List     []*LoginRecord `json:"list"`
	Total    int64          `json:"total"`
	Page     int32          `json:"page"`
	PageSize int32          `json:"pageSize"`
}
