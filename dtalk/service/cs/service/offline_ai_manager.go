package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/inconshreveable/log15"

	"github.com/web3chat/txchat/dtalk/service/cs/model"
)

// offlineAIManager 按 PollInterval 扫描 DB：仅 status=离线 的客服启动离线 AI Worker；
// 客服上线（或非离线）后下一轮 reconcile 会 cancel 对应 Worker，降低与 cs-web 双开同一 Agent 的风险。
type offlineAIManager struct {
	svc *Service
	log log15.Logger

	mu    sync.Mutex
	stops map[string]context.CancelFunc
}

func (s *Service) StartOfflineAI(ctx context.Context) {
	if s.cfg.OfflineAI == nil || !s.cfg.OfflineAI.Enable {
		return
	}
	oa := s.cfg.OfflineAI
	if strings.TrimSpace(oa.DeepSeekAPIKey) == "" {
		s.log.Error("OfflineAI.Enable=true 但未配置 DeepSeekAPIKey，离线 AI 未启动")
		return
	}
	if strings.TrimSpace(oa.CometURL) == "" {
		s.log.Error("OfflineAI.Enable=true 但未配置 CometURL，离线 AI 未启动")
		return
	}
	if oa.DebugIgnoreAgentStatus {
		s.log.Warn("离线 AI：DebugIgnoreAgentStatus=true，将为全部 cs_agent 启动 Worker（可能与 cs-web 双连冲突，仅用于联调）")
	}
	mgr := &offlineAIManager{
		svc:   s,
		log:   log15.New("module", "cs/offline-ai-mgr"),
		stops: make(map[string]context.CancelFunc),
	}
	go mgr.run(ctx)
}

func (m *offlineAIManager) pollInterval() time.Duration {
	poll := time.Duration(m.svc.cfg.OfflineAI.PollInterval)
	if poll <= 0 {
		return 15 * time.Second
	}
	return poll
}

func (m *offlineAIManager) run(ctx context.Context) {
	m.log.Info("离线 AI 管理器已启动", "poll", m.pollInterval())

	ticker := time.NewTicker(m.pollInterval())
	defer ticker.Stop()

	m.reconcile(ctx)

	for {
		select {
		case <-ctx.Done():
			m.stopAll()
			m.log.Info("离线 AI 管理器已退出", "reason", ctx.Err())
			return
		case <-ticker.C:
			m.reconcile(ctx)
		}
	}
}

func (m *offlineAIManager) reconcile(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	agents, err := m.svc.dao.GetAgentList()
	if err != nil {
		m.log.Error("离线 AI：查询客服列表失败", "err", err)
		return
	}

	want := make(map[string]struct{})
	debugIgnore := m.svc.cfg.OfflineAI != nil && m.svc.cfg.OfflineAI.DebugIgnoreAgentStatus
	offlineSeen := 0
	for _, a := range agents {
		if a == nil {
			continue
		}
		if debugIgnore {
			want[a.AgentID] = struct{}{}
			continue
		}
		if a.Status != model.AgentStatusOffline {
			continue
		}
		offlineSeen++
		want[a.AgentID] = struct{}{}
	}

	m.log.Info("离线 AI：reconcile 扫描完成",
		"agents_total", len(agents),
		"offline_db_status", offlineSeen,
		"want_workers", len(want),
		"DebugIgnoreAgentStatus", debugIgnore,
	)

	m.mu.Lock()
	defer m.mu.Unlock()

	for id, cancel := range m.stops {
		if _, ok := want[id]; !ok {
			m.log.Info("离线 AI：停止 Worker（客服已非离线）", "agentID", id)
			cancel()
			delete(m.stops, id)
		}
	}

	for id := range want {
		if _, exists := m.stops[id]; exists {
			continue
		}
		child, cancel := context.WithCancel(ctx)
		m.stops[id] = cancel
		w := newOfflineWorker(m.svc, id, m.svc.cfg.OfflineAI)
		m.log.Info("离线 AI：启动 Worker", "agentID", id)
		go w.Run(child)
	}
}

func (m *offlineAIManager) stopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, cancel := range m.stops {
		m.log.Info("离线 AI：取消 Worker", "agentID", id)
		cancel()
		delete(m.stops, id)
	}
}
