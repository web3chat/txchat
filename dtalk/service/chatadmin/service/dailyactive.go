package service

import (
	"context"
	"fmt"
	"time"

	"github.com/web3chat/txchat/dtalk/service/chatadmin/model/biz"
)

const dailyActiveSnapshotInterval = 30 * time.Minute

// SnapshotDailyActive 统计并入库指定自然日的活跃数据（upsert）
func (s *Service) SnapshotDailyActive(ctx context.Context, day time.Time) (*biz.DailyActiveRecord, error) {
	day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	statDate := day.Format("2006-01-02")

	activeUsers, err := s.groupInfoDAO.GetActiveUserCountByDay(ctx, day)
	if err != nil {
		fmt.Printf("[Service] SnapshotDailyActive GetActiveUserCountByDay failed: statDate=%s err=%v\n", statDate, err)
		return nil, fmt.Errorf("query active users failed: %w", err)
	}
	activeGroups, err := s.groupInfoDAO.GetActiveGroupCountByDay(ctx, day)
	if err != nil {
		fmt.Printf("[Service] SnapshotDailyActive GetActiveGroupCountByDay failed: statDate=%s err=%v\n", statDate, err)
		return nil, fmt.Errorf("query active groups failed: %w", err)
	}

	var totalUsers, totalGroups int64
	if n, e := s.groupInfoDAO.GetTotalUserCount(ctx); e == nil {
		totalUsers = n
	} else {
		fmt.Printf("[Service] SnapshotDailyActive GetTotalUserCount failed: statDate=%s err=%v\n", statDate, e)
	}
	if n, e := s.groupInfoDAO.GetTotalGroupCount(ctx); e == nil {
		totalGroups = n
	} else {
		fmt.Printf("[Service] SnapshotDailyActive GetTotalGroupCount failed: statDate=%s err=%v\n", statDate, e)
	}

	rec := &biz.DailyActiveRecord{
		StatDate:     statDate,
		ActiveUsers:  activeUsers,
		ActiveGroups: activeGroups,
		TotalUsers:   totalUsers,
		TotalGroups:  totalGroups,
		UpdatedAt:    time.Now().UnixMilli(),
	}
	keepCumulative := day.Before(time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, day.Location()))
	if err := s.dailyActiveDAO.Upsert(ctx, rec, keepCumulative); err != nil {
		fmt.Printf("[Service] SnapshotDailyActive Upsert failed: statDate=%s err=%v\n", statDate, err)
		return nil, err
	}
	fmt.Printf("[Service] SnapshotDailyActive ok: statDate=%s activeUsers=%d activeGroups=%d totalUsers=%d totalGroups=%d\n",
		statDate, activeUsers, activeGroups, totalUsers, totalGroups)
	return rec, nil
}

// ListDailyActive 查询每日活跃入库记录
func (s *Service) ListDailyActive(ctx context.Context, req *biz.DailyActiveListRequest) (*biz.DailyActiveListResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 30
	}
	if pageSize > 200 {
		pageSize = 200
	}
	list, total, err := s.dailyActiveDAO.List(ctx, req.StartDate, req.EndDate, req.Dates, page, pageSize)
	if err != nil {
		fmt.Printf("[Service] ListDailyActive failed: startDate=%s endDate=%s dates=%v page=%d err=%v\n",
			req.StartDate, req.EndDate, req.Dates, page, err)
		return nil, err
	}
	return &biz.DailyActiveListResponse{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// StartDailyActiveRecorder 启动后台定时入库（今日快照；同时补写昨日）
func (s *Service) StartDailyActiveRecorder(ctx context.Context) {
	go func() {
		if err := s.groupInfoDAO.EnsureActiveStatIndexes(context.Background()); err != nil {
			fmt.Printf("[Service] EnsureActiveStatIndexes failed: err=%v\n", err)
		}
		s.runDailyActiveSnapshot("startup")
		ticker := time.NewTicker(dailyActiveSnapshotInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				fmt.Printf("[Service] DailyActiveRecorder stopped: reason=%v\n", ctx.Err())
				return
			case <-ticker.C:
				s.runDailyActiveSnapshot("ticker")
			}
		}
	}()
}

func (s *Service) runDailyActiveSnapshot(trigger string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	now := time.Now()
	if _, err := s.SnapshotDailyActive(ctx, now); err != nil {
		fmt.Printf("[Service] runDailyActiveSnapshot today failed: trigger=%s err=%v\n", trigger, err)
	}
	yesterday := now.AddDate(0, 0, -1)
	if _, err := s.SnapshotDailyActive(ctx, yesterday); err != nil {
		fmt.Printf("[Service] runDailyActiveSnapshot yesterday failed: trigger=%s err=%v\n", trigger, err)
	}
}

func clampActiveDetailPage(page, pageSize int32) (int32, int32) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

// ListActiveAddresses 分页返回某一天的活跃地址。
func (s *Service) ListActiveAddresses(ctx context.Context, day time.Time, page, pageSize int32) (*biz.ActiveAddressPage, error) {
	page, pageSize = clampActiveDetailPage(page, pageSize)
	statDate := day.Format("2006-01-02")
	list, total, err := s.groupInfoDAO.ListActiveUserIDsByDay(ctx, day, page, pageSize)
	if err != nil {
		fmt.Printf("[Service] ListActiveAddresses failed: date=%s page=%d pageSize=%d err=%v\n",
			statDate, page, pageSize, err)
		return nil, err
	}
	if list == nil {
		list = []string{}
	}
	return &biz.ActiveAddressPage{
		Date:     statDate,
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// ListActiveGroups 分页返回某一天有消息的群名。
func (s *Service) ListActiveGroups(ctx context.Context, day time.Time, page, pageSize int32) (*biz.ActiveGroupPage, error) {
	page, pageSize = clampActiveDetailPage(page, pageSize)
	statDate := day.Format("2006-01-02")
	list, total, err := s.groupInfoDAO.ListActiveGroupsByDay(ctx, day, page, pageSize)
	if err != nil {
		fmt.Printf("[Service] ListActiveGroups failed: date=%s page=%d pageSize=%d err=%v\n",
			statDate, page, pageSize, err)
		return nil, err
	}
	if list == nil {
		list = []*biz.ActiveGroupItem{}
	}
	return &biz.ActiveGroupPage{
		Date:     statDate,
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}
