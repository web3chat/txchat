package dao

import "time"

// 与 chatclean 全局保留一致。epoch 同 pkg/util/snowflake.go。
const (
	retentionKeep        = 168 * time.Hour
	retentionEpoch int64 = 1590940800000
	retentionShift       = 22
)

// retainExpired 在 create_time 和 mid 都早于 now-168h 时为 true。
// createTime 为 0 不当作过期，避免时间缺失的消息被直接丢掉。
//
// 群聊正文目前没有联邦落库（DeliverGroup 只推送，不写 dtalk_group_msg_*）。
// 私聊的 StoreMirroredPrivate 和 AppendFederatedMsg 都走这个判断。
// 以后若增加群聊镜像写入，用同一个函数，过期则当成功返回、不插入。
func retainExpired(mid int64, createTime uint64, now time.Time) bool {
	if mid <= 0 || createTime == 0 {
		return false
	}
	cutoffMs := now.Add(-retentionKeep).UnixMilli()
	if int64(createTime) >= cutoffMs {
		return false
	}
	delta := cutoffMs - retentionEpoch
	if delta <= 0 {
		return false
	}
	cutoffMid := delta << retentionShift
	return mid < cutoffMid
}
