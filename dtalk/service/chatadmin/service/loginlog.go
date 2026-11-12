package service

import (
	"context"
	"fmt"
	"time"
)

const (
	loginLogRetentionMonths = 2
	loginLogPurgeInterval   = 24 * time.Hour
	loginLogPurgeTimeout    = 5 * time.Minute
)

// loginLogRetentionCutoff 登录日志保留截止点：早于该时间的记录删除。
func loginLogRetentionCutoff(now time.Time) time.Time {
	return now.AddDate(0, -loginLogRetentionMonths, 0)
}

// StartLoginLogRetention 启动时清一次，之后每天清一次超过两个月的登录日志。
func (s *Service) StartLoginLogRetention(ctx context.Context) {
	go func() {
		s.purgeExpiredLoginLogs("startup")
		ticker := time.NewTicker(loginLogPurgeInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				fmt.Printf("[Service] LoginLogRetention stopped: reason=%v\n", ctx.Err())
				return
			case <-ticker.C:
				s.purgeExpiredLoginLogs("ticker")
			}
		}
	}()
}

func (s *Service) purgeExpiredLoginLogs(trigger string) {
	if s.userLoginDAO == nil {
		fmt.Printf("[Service] purgeExpiredLoginLogs skipped: dao nil trigger=%s\n", trigger)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), loginLogPurgeTimeout)
	defer cancel()
	cutoff := loginLogRetentionCutoff(time.Now())
	deleted, err := s.userLoginDAO.DeleteLoginLogsBefore(ctx, cutoff.UnixMilli())
	if err != nil {
		fmt.Printf("[Service] purgeExpiredLoginLogs failed: trigger=%s cutoff=%s deleted=%d err=%v\n",
			trigger, cutoff.Format(time.RFC3339), deleted, err)
		return
	}
	fmt.Printf("[Service] purgeExpiredLoginLogs ok: trigger=%s cutoff=%s deleted=%d\n",
		trigger, cutoff.Format(time.RFC3339), deleted)
}
