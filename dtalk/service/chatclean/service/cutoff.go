package service

import "time"

// snowflakeEpoch 与 pkg/util/snowflake.go 的 epoch 一致：2020-06-01 00:00:00 UTC。
const snowflakeEpoch int64 = 1590940800000

const snowflakeTimeShift = 22

// Cutoff 返回 now-keep 的毫秒，以及同一时刻的 snowflake 下界。
// keep <= 0 时按 168 小时。cutoff 不晚于 epoch 时 cutoffMid 为 0。
func Cutoff(now time.Time, keep time.Duration) (cutoffMs, cutoffMid int64) {
	if keep <= 0 {
		keep = defaultKeep
	}
	cutoffMs = now.Add(-keep).UnixMilli()
	delta := cutoffMs - snowflakeEpoch
	if delta > 0 {
		cutoffMid = delta << snowflakeTimeShift
	}
	return cutoffMs, cutoffMid
}

// CutoffDay 是 now-keep 在服务器本地时区的 yyyyMMdd。这一天的目录保留。
func CutoffDay(now time.Time, keep time.Duration) string {
	if keep <= 0 {
		keep = defaultKeep
	}
	return now.Add(-keep).Format("20060102")
}

// DateFolderExpired 只接受 8 位数字目录名，且必须严格早于 cutoffDay。
func DateFolderExpired(name, cutoffDay string) bool {
	return isDateFolder(name) && isDateFolder(cutoffDay) && name < cutoffDay
}

func isDateFolder(name string) bool {
	if len(name) != 8 {
		return false
	}
	for _, c := range name {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// expiredMessage 要求 mid 和 create_time 同时早于截止线。任一不满足就保留。
// createTime <= 0 视为时间未知，不删。
func expiredMessage(mid, createTime, cutoffMid, cutoffMs int64) bool {
	if mid <= 0 || createTime <= 0 {
		return false
	}
	return mid < cutoffMid && createTime < cutoffMs
}
