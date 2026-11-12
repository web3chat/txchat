package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/inconshreveable/log15"

	comet "github.com/web3chat/txchat/im/api/comet/grpc"

	"github.com/web3chat/txchat/dtalk/service/cs/config"
)

type chatTurn struct {
	role    string
	content string
}

type peerChat struct {
	mu     sync.Mutex
	turns  []chatTurn
	paused bool
}

type offlineSession struct {
	conn    *websocket.Conn
	writeMu *sync.Mutex
	seq     *atomic.Int32
	log     log15.Logger
}

func (s *offlineSession) send(p *comet.Proto) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return p.WriteWebsocket2(s.conn)
}

func (s *offlineSession) sendChatText(agentID, peer, content string) error {
	body, err := marshalSendTextPayload(agentID, peer, content)
	if err != nil {
		s.log.Error("离线 AI：构造 SendMsg 载荷失败", "peer", peer, "err", err)
		return err
	}
	p := &comet.Proto{
		Ver:  1,
		Op:   int32(comet.Op_SendMsg),
		Seq:  s.seq.Add(1),
		Body: body,
	}
	if err := s.send(p); err != nil {
		s.log.Error("离线 AI：SendMsg 写入失败", "peer", peer, "err", err)
		return err
	}
	s.log.Info("离线 AI：SendMsg 已发送", "peer", peer, "chars", len(content))
	return nil
}

type offlineWorker struct {
	svc     *Service
	oa      *config.OfflineAIConfig
	agentID string
	log     log15.Logger

	ctxMu     sync.RWMutex
	parentCtx context.Context

	sessMu sync.RWMutex
	sess   *offlineSession

	histories  sync.Map // peer address -> *peerChat
	peerTimers sync.Map // peer -> *time.Timer
}

func newOfflineWorker(svc *Service, agentID string, oa *config.OfflineAIConfig) *offlineWorker {
	return &offlineWorker{
		svc:     svc,
		oa:      oa,
		agentID: agentID,
		log:     log15.New("module", "cs/offline-ai", "agentID", agentID),
	}
}

func (w *offlineWorker) setParentCtx(ctx context.Context) {
	w.ctxMu.Lock()
	defer w.ctxMu.Unlock()
	w.parentCtx = ctx
}

func (w *offlineWorker) aiContext() context.Context {
	w.ctxMu.RLock()
	defer w.ctxMu.RUnlock()
	if w.parentCtx == nil {
		return context.Background()
	}
	return w.parentCtx
}

func (w *offlineWorker) currentSession() *offlineSession {
	w.sessMu.RLock()
	defer w.sessMu.RUnlock()
	return w.sess
}

func (w *offlineWorker) attachSession(s *offlineSession) {
	w.sessMu.Lock()
	defer w.sessMu.Unlock()
	w.sess = s
}

func (w *offlineWorker) detachSession(want *offlineSession) {
	w.sessMu.Lock()
	defer w.sessMu.Unlock()
	if w.sess == want {
		w.sess = nil
	}
}

func (w *offlineWorker) stopPeerTimers() {
	w.peerTimers.Range(func(key, value any) bool {
		if t, ok := value.(*time.Timer); ok {
			t.Stop()
		}
		w.peerTimers.Delete(key)
		return true
	})
}

func (w *offlineWorker) debounceMillis() time.Duration {
	ms := w.oa.DebounceMillis
	if ms <= 0 {
		ms = 650
	}
	return time.Duration(ms) * time.Millisecond
}

func (w *offlineWorker) introDelay() time.Duration {
	ms := w.oa.IntroDelayMillis
	if ms <= 0 {
		ms = 150
	}
	return time.Duration(ms) * time.Millisecond
}

// receiveMsgAckDelay 与 cs.toml CometReceiveAckDelayMillis、cs-web 使用同一服务端口径。
func (w *offlineWorker) receiveMsgAckDelay() time.Duration {
	ms := w.svc.CometReceiveAckDelayMillis()
	return time.Duration(ms) * time.Millisecond
}

func (w *offlineWorker) getPeer(peer string) *peerChat {
	if v, ok := w.histories.Load(peer); ok {
		return v.(*peerChat)
	}
	pc := &peerChat{}
	actual, _ := w.histories.LoadOrStore(peer, pc)
	return actual.(*peerChat)
}

func (w *offlineWorker) Run(parent context.Context) {
	backoff := time.Second
	maxB := time.Duration(w.oa.ReconnectBackoffMax)
	if maxB <= 0 {
		maxB = 30 * time.Second
	}
	for {
		select {
		case <-parent.Done():
			w.stopPeerTimers()
			return
		default:
		}
		w.setParentCtx(parent)
		runCtx, cancel := context.WithCancel(parent)
		err := w.runSession(runCtx, &backoff)
		cancel()
		w.stopPeerTimers()
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		if parent.Err() != nil {
			return
		}
		w.log.Warn("离线 AI：会话结束，准备重连", "err", err)
		sleep := backoff
		if sleep > maxB {
			sleep = maxB
		}
		select {
		case <-parent.Done():
			return
		case <-time.After(sleep):
		}
		if backoff < maxB {
			nb := backoff * 2
			if nb > maxB {
				backoff = maxB
			} else {
				backoff = nb
			}
		}
	}
}

func (w *offlineWorker) runSession(ctx context.Context, backoff *time.Duration) error {
	tok, err := w.svc.GenerateFZMSignature(w.agentID)
	if err != nil {
		w.log.Error("离线 AI：GenerateFZMSignature 失败", "agentID", w.agentID, "err", err)
		return err
	}
	seq := &atomic.Int32{}
	conn, err := dialCometAndAuth(ctx, w.log, w.oa, w.svc.cfg.AppId, tok.FZMSignature, seq)
	if err != nil {
		w.log.Warn("离线 AI：Comet 拨号或认证失败", "err", err)
		return err
	}
	if backoff != nil {
		*backoff = time.Second
	}
	defer conn.Close()

	sess := &offlineSession{
		conn:    conn,
		writeMu: &sync.Mutex{},
		seq:     seq,
		log:     w.log,
	}
	w.attachSession(sess)
	defer w.detachSession(sess)

	hbCtx, hbCancel := context.WithCancel(ctx)
	defer hbCancel()

	hbSec := w.oa.HeartbeatSec
	if hbSec <= 0 {
		hbSec = 28
	}
	go runHeartbeat(hbCtx, seq, hbSec, w.log, sess.send)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		p := new(comet.Proto)
		if err := p.ReadWebsocket2(conn); err != nil {
			w.log.Warn("离线 AI：ReadWebsocket2 失败", "err", err)
			return err
		}
		switch comet.Op(p.Op) {
		case comet.Op_HeartbeatReply:
			continue
		case comet.Op_SendMsgReply:
			continue
		case comet.Op_ReceiveMsg:
			w.log.Info("离线 AI：收到 ReceiveMsg", "seq", p.Seq, "bodyLen", len(p.Body))
			seqCopy := p.Seq
			sessAtRecv := sess
			delay := w.receiveMsgAckDelay()
			go func() {
				timer := time.NewTimer(delay)
				defer timer.Stop()
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
				}
				cur := w.currentSession()
				if cur == nil || cur != sessAtRecv {
					w.log.Warn("离线 AI：延迟 ReceiveMsgReply 时会话已切换，跳过 Ack", "seq", seqCopy)
					return
				}
				ack := &comet.Proto{
					Ver: 1,
					Op:  int32(comet.Op_ReceiveMsgReply),
					Seq: seqCopy,
					Ack: seqCopy,
				}
				if err := cur.send(ack); err != nil {
					w.log.Warn("离线 AI：ReceiveMsgReply 发送失败", "seq", seqCopy, "delay", delay, "err", err)
					return
				}
				w.log.Debug("离线 AI：ReceiveMsgReply 已发送", "seq", seqCopy, "delay", delay)
			}()
			from, text, skip := decodeInboundUserText(w.agentID, w.svc, w.log, p.Body)
			if skip {
				continue
			}
			if strings.TrimSpace(text) == "" {
				w.log.Warn("离线 AI：解析后无可回复文本（可能是解密失败或非文本消息），已跳过", "from", from)
				continue
			}
			w.log.Info("离线 AI：将调度自动回复", "from", from, "chars", len(text))
			w.schedulePeer(from, text)
		default:
			continue
		}
	}
}

func (w *offlineWorker) schedulePeer(peer, text string) {
	if old, ok := w.peerTimers.Load(peer); ok {
		old.(*time.Timer).Stop()
	}
	peerCopy := peer
	textCopy := text
	d := w.debounceMillis()
	t := time.AfterFunc(d, func() {
		w.peerTimers.Delete(peerCopy)
		w.processPeer(peerCopy, textCopy)
	})
	w.peerTimers.Store(peer, t)
}

func (w *offlineWorker) buildDSHistory(peer string) []dsChatMsg {
	pc := w.getPeer(peer)
	pc.mu.Lock()
	defer pc.mu.Unlock()
	out := make([]dsChatMsg, 0, len(pc.turns))
	for _, t := range pc.turns {
		role := "user"
		if t.role == "assistant" {
			role = "assistant"
		}
		c := strings.TrimSpace(t.content)
		if c == "" {
			continue
		}
		out = append(out, dsChatMsg{Role: role, Content: c})
	}
	return out
}

func (w *offlineWorker) processPeer(peer, userText string) {
	ctx := w.aiContext()
	if ctx.Err() != nil {
		w.log.Info("离线 AI：父上下文已取消，跳过本次回复", "peer", peer)
		return
	}
	if strings.TrimSpace(userText) == "" {
		return
	}

	ses := w.currentSession()
	if ses == nil {
		w.log.Warn("离线 AI：无活跃 Comet 会话，忽略消息", "peer", peer)
		return
	}

	pc := w.getPeer(peer)
	pc.mu.Lock()
	if pc.paused {
		pc.mu.Unlock()
		w.log.Info("离线 AI：会话已转人工暂停，跳过自动回复", "peer", peer)
		return
	}
	pc.turns = append(pc.turns, chatTurn{role: "user", content: userText})
	first := len(pc.turns) == 1
	pc.mu.Unlock()

	sys := buildOfflineAISystemPrompt(w.oa)
	intro := proactiveIntroMarkdown(w.oa)

	if first && strings.TrimSpace(intro) != "" {
		if err := ses.sendChatText(w.agentID, peer, intro); err != nil {
			return
		}
		pc.mu.Lock()
		pc.turns = append(pc.turns, chatTurn{role: "assistant", content: intro})
		pc.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-time.After(w.introDelay()):
		}
	}

	hist := w.buildDSHistory(peer)
	replyRaw, err := deepSeekChat(ctx, w.oa, w.log, sys, hist)
	if err != nil {
		w.log.Error("离线 AI：DeepSeek 调用失败", "peer", peer, "err", err)
		return
	}
	out, human := stripHandoffMarker(replyRaw)
	if strings.TrimSpace(out) != "" {
		ses2 := w.currentSession()
		if ses2 == nil {
			w.log.Warn("离线 AI：发送模型回复时会话已断开", "peer", peer)
			return
		}
		if err := ses2.sendChatText(w.agentID, peer, out); err != nil {
			return
		}
		pc.mu.Lock()
		pc.turns = append(pc.turns, chatTurn{role: "assistant", content: out})
		pc.mu.Unlock()
	}
	if human {
		pc.mu.Lock()
		pc.paused = true
		pc.mu.Unlock()
		w.log.Warn("离线 AI：模型标记转人工，暂停该用户自动回复", "peer", peer)
	}
}