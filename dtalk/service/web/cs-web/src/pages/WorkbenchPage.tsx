import { useCallback, useEffect, useRef, useState } from 'react';
import { connectionStore, ChatMessage, LoginResult } from '../store';
import { loadChatState, nextMsgCounterSeed, saveChatState, STORAGE_VERSION } from '../lib/chatPersistence';
import { clearPersistedSession } from '../lib/sessionPersistence';
import { updateAgentStatus, setSessionToken, chainPing, bindCometOnChain, decryptMsg, registerPubKeyOnChain } from '../lib/api';
import { Op, Proto } from '../lib/protocol';
import {
  parseReceivedMessage, buildSendTextMessage,
  MsgType, decodeBizProto, EventType, decodeCommonAck,
  uint8ToBase64, base64ToUint8, decodeTextMsg, decodeCommon,
} from '../lib/xproto';
import type { ConnectionState, IMConnection } from '../lib/ws';
import { requestDeepseekReply } from '../lib/deepseek';
import {
  AiPolicyState,
  DEFAULT_AI_POLICY,
  buildFullSystemPrompt,
  loadAiPolicy,
  loadAiPausedUsers,
  parseAiReply,
  saveAiPolicy,
  saveAiPausedUsers,
  getProactiveIntroMarkdown,
} from '../lib/aiPolicy';
import { ChatMarkdown } from '../components/ChatMarkdown';

const AI_DEBOUNCE_MS = 650;
const INTRO_THEN_AI_DELAY_MS = 150;
const AI_LS_PREFIX = 'cs_web_ai_auto:v1:';

const FALLBACK_COMET_RECEIVE_ACK_DELAY_MS = 280;

function effectiveReceiveAckDelayMs(lr: LoginResult | null): number {
  const v = lr?.cometReceiveAckDelayMillis;
  if (typeof v === 'number' && v > 0) return v;
  return FALLBACK_COMET_RECEIVE_ACK_DELAY_MS;
}

function scheduleDelayedReceiveMsgAck(conn: IMConnection, seq: number, delayMs: number) {
  window.setTimeout(() => {
    if (connectionStore.connection !== conn) return;
    conn.sendAck(seq);
  }, delayMs);
}

interface Props {
  agentName: string;
  onLogout: () => void;
}

const stateLabels: Record<ConnectionState, { text: string; color: string }> = {
  disconnected:  { text: 'Comet 已断开', color: '#e74c3c' },
  connecting:    { text: 'Comet 连接中...', color: '#f39c12' },
  authenticating:{ text: 'Comet 认证中...', color: '#f39c12' },
  connected:     { text: 'Comet 已连接', color: '#27ae60' },
  reconnecting:  { text: 'Comet 重连中...', color: '#f39c12' },
  error:         { text: 'Comet 连接错误', color: '#e74c3c' },
};

function bootstrapLocalChat(): {
  conversations: Record<string, ChatMessage[]>;
  activeUserId: string | null;
  msgCounterSeed: number;
} {
  const aid = connectionStore.loginResult?.csLogin.agent.agentId ?? '';
  if (!aid) {
    return { conversations: {}, activeUserId: null, msgCounterSeed: 0 };
  }
  const persisted = loadChatState(aid);
  const conversations = persisted?.conversations ?? {};
  const activeUserId =
    persisted?.activeUserId && conversations[persisted.activeUserId]
      ? persisted.activeUserId
      : null;
  return {
    conversations,
    activeUserId,
    msgCounterSeed: nextMsgCounterSeed(conversations),
  };
}

export default function WorkbenchPage({ agentName, onLogout }: Props) {
  const bootRef = useRef<ReturnType<typeof bootstrapLocalChat> | null>(null);
  if (bootRef.current === null) {
    bootRef.current = bootstrapLocalChat();
  }
  const boot = bootRef.current;

  const [conversations, setConversations] = useState<Record<string, ChatMessage[]>>(boot.conversations);
  const [activeUserId, setActiveUserId] = useState<string | null>(boot.activeUserId);
  const [inputText, setInputText] = useState('');
  const [unread, setUnread] = useState<Record<string, number>>({});
  const [connState, setConnState] = useState<ConnectionState>('connected');
  const [connDetail, setConnDetail] = useState('');
  const [showLoginResult, setShowLoginResult] = useState(() => Object.keys(boot.conversations).length === 0);
  const [loginResult] = useState<LoginResult | null>(connectionStore.loginResult);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const msgIdCounter = useRef(boot.msgCounterSeed);
  const agentAddr = loginResult?.csLogin.agent.agentId ?? '';

  const [chainRpcInput, setChainRpcInput] = useState(
    () => connectionStore.connectParams?.chainRpcUrl || 'http://localhost:8901',
  );
  const [chainPingHeight, setChainPingHeight] = useState<number | null>(null);
  const [chainBusy, setChainBusy] = useState(false);
  const [chainMsg, setChainMsg] = useState('');
  const [aiAutoReply, setAiAutoReply] = useState(() => {
    if (!agentAddr || typeof localStorage === 'undefined') return false;
    return localStorage.getItem(`${AI_LS_PREFIX}${agentAddr}`) === '1';
  });
  const [aiPolicy, setAiPolicy] = useState<AiPolicyState>(() =>
    agentAddr ? loadAiPolicy(agentAddr) : { ...DEFAULT_AI_POLICY },
  );
  const [aiPausedForUsers, setAiPausedForUsers] = useState<Record<string, boolean>>(() =>
    agentAddr ? loadAiPausedUsers(agentAddr) : {},
  );
  const [showAiPolicyEditor, setShowAiPolicyEditor] = useState(false);

  const conversationsRef = useRef(conversations);
  const aiAutoReplyRef = useRef(aiAutoReply);
  const aiPolicyRef = useRef(aiPolicy);
  const aiPausedRef = useRef(aiPausedForUsers);
  const aiDebounceTimersRef = useRef<Record<string, ReturnType<typeof setTimeout>>>({});
  /** 客服「结束会话」后拦截出站消息（含异步 AI），防止发送时又重建会话 */
  const suppressOutboundRef = useRef<Set<string>>(new Set());

  useEffect(() => {
    conversationsRef.current = conversations;
  }, [conversations]);

  useEffect(() => {
    aiAutoReplyRef.current = aiAutoReply;
  }, [aiAutoReply]);

  useEffect(() => {
    aiPolicyRef.current = aiPolicy;
  }, [aiPolicy]);

  useEffect(() => {
    aiPausedRef.current = aiPausedForUsers;
  }, [aiPausedForUsers]);

  useEffect(() => {
    if (!agentAddr || typeof localStorage === 'undefined') return;
    localStorage.setItem(`${AI_LS_PREFIX}${agentAddr}`, aiAutoReply ? '1' : '0');
  }, [agentAddr, aiAutoReply]);

  useEffect(() => {
    if (!agentAddr) return;
    saveAiPausedUsers(agentAddr, aiPausedForUsers);
  }, [agentAddr, aiPausedForUsers]);

  useEffect(() => {
    return () => {
      const timers = aiDebounceTimersRef.current;
      for (const k of Object.keys(timers)) {
        clearTimeout(timers[k]);
        delete timers[k];
      }
    };
  }, []);

  useEffect(() => {
    const token = loginResult?.csLogin.sessionToken;
    if (token) {
      setSessionToken(token);
      registerPubKeyOnChain()
        .then((r) => console.log('[CS-WS] agent 公钥已注册到链上', r))
        .catch(() => { /* 注册失败（接口未部署或已注册），忽略 */ });
    }
  }, [loginResult]);

  /** 进入工作台即标为在线：防止恢复会话时接口失败导致 DB 仍为离线，服务端离线 AI 继续代答。 */
  useEffect(() => {
    let alive = true;
    void (async () => {
      try {
        await updateAgentStatus(1);
        if (alive) console.log('[CS-Workbench] 已与服务器同步「在线」状态');
      } catch (e) {
        console.error(
          '[CS-Workbench] 同步「在线」失败：若数据库仍为离线，离线 AI 可能继续代答。请检查网关 POST /cs/agent/status。',
          e,
        );
      }
    })();
    return () => {
      alive = false;
    };
  }, []);

  // 本地持久化：按 Agent ID 分桶，刷新或下次打开同一客服账号仍可看到历史会话
  useEffect(() => {
    if (!agentAddr) return;
    saveChatState(agentAddr, {
      version: STORAGE_VERSION,
      conversations,
      activeUserId,
    });
  }, [agentAddr, conversations, activeUserId]);

  useEffect(() => {
    const conn = connectionStore.connection;
    if (!conn) {
      setConnState('disconnected');
      return;
    }
    setConnState(conn.state);
    conn.setStateChangeHandler((state, detail) => {
      setConnState(state);
      setConnDetail(detail || '');
    });
    conn.setDisconnectHandler(() => {
      // 自动重连由 IMConnection 内部处理，此处不做 onLogout
    });
  }, []);

  // 获取按最后消息时间排序的用户列表
  const sortedUserIds = Object.keys(conversations).sort((a, b) => {
    const msgsA = conversations[a];
    const msgsB = conversations[b];
    const lastA = msgsA[msgsA.length - 1]?.timestamp || 0;
    const lastB = msgsB[msgsB.length - 1]?.timestamp || 0;
    return lastB - lastA;
  });

  const addMessage = useCallback((userId: string, msg: ChatMessage) => {
    setConversations((prev) => {
      const list = prev[userId] || [];
      return { ...prev, [userId]: [...list, msg] };
    });
  }, []);

  const sendTextToPeer = useCallback(
    (peerUserId: string, content: string) => {
      if (suppressOutboundRef.current.has(peerUserId)) {
        console.warn('[CS-Workbench] 会话已结束，跳过向对方发送', { peerLen: peerUserId.length });
        return false;
      }
      const conn = connectionStore.connection;
      if (!conn || conn.state !== 'connected') {
        console.warn('[CS-AI] 跳过发送：Comet 未连接');
        return false;
      }
      const myAddr = loginResult?.csLogin.agent.agentId ?? '';
      if (!myAddr) return false;
      const trimmed = content.trim();
      if (!trimmed) return false;
      try {
        const body = buildSendTextMessage({
          from: myAddr,
          target: peerUserId,
          content: trimmed,
        });
        conn.sendMsg(body);
      } catch (err) {
        console.error('[CS-WS] 发送消息失败', err);
        return false;
      }
      const msg: ChatMessage = {
        id: `msg-${Date.now()}-${msgIdCounter.current++}`,
        from: myAddr,
        to: peerUserId,
        content: trimmed,
        timestamp: Date.now(),
        isSelf: true,
      };
      addMessage(peerUserId, msg);
      return true;
    },
    [addMessage, loginResult?.csLogin.agent.agentId],
  );

  const scheduleAiAutoReply = useCallback(
    (userId: string) => {
      const timers = aiDebounceTimersRef.current;
      if (timers[userId]) {
        clearTimeout(timers[userId]);
      }
      timers[userId] = setTimeout(() => {
        delete timers[userId];
        void (async () => {
          if (!aiAutoReplyRef.current) return;
          if (aiPausedRef.current[userId]) return;
          const conn = connectionStore.connection;
          if (!conn || conn.state !== 'connected') {
            console.warn('[CS-AI] 跳过自动生成：连接不可用');
            return;
          }

          let history = conversationsRef.current[userId];
          if (!history?.length) return;
          if (suppressOutboundRef.current.has(userId)) return;

          let last = history[history.length - 1];
          if (last.isSelf) return;

          const introMd = getProactiveIntroMarkdown(aiPolicyRef.current);
          const isFirstUserTurn = history.length === 1;
          let sentProactiveIntro = false;

          if (isFirstUserTurn && introMd) {
            sentProactiveIntro = true;
            sendTextToPeer(userId, introMd);
            await new Promise((r) => setTimeout(r, INTRO_THEN_AI_DELAY_MS));
            if (!aiAutoReplyRef.current || aiPausedRef.current[userId]) return;
            if (suppressOutboundRef.current.has(userId)) return;
            history = conversationsRef.current[userId];
            if (!history?.length) return;
            last = history[history.length - 1];
          }

          if (last.isSelf && !sentProactiveIntro) return;

          try {
            if (suppressOutboundRef.current.has(userId)) return;
            const histNow = conversationsRef.current[userId];
            if (!histNow?.length) return;
            const tail = histNow[histNow.length - 1];
            if (!sentProactiveIntro && tail.isSelf) return;

            const systemPrompt = buildFullSystemPrompt(aiPolicyRef.current);
            const rawReply = await requestDeepseekReply(histNow, {
              systemPrompt,
              temperature: 0.45,
            });
            const { textToSend, needsHuman } = parseAiReply(rawReply);
            if (!textToSend.trim() && !needsHuman) return;
            if (textToSend.trim()) {
              sendTextToPeer(userId, textToSend);
            }
            if (needsHuman) {
              console.warn('[CS-AI] 模型触发转人工标记，已暂停该会话自动回复', {
                peerLen: userId.length,
              });
              setAiPausedForUsers((prev) => ({ ...prev, [userId]: true }));
            }
          } catch (e) {
            console.error('[CS-AI] DeepSeek 自动回复失败', e);
          }
        })();
      }, AI_DEBOUNCE_MS);
    },
    [sendTextToPeer],
  );

  const handleCloseConversation = useCallback((userId: string) => {
    if (
      !window.confirm(
        '确定结束与该用户的会话？本地聊天记录将被清除；对方再次发消息时会重新出现在列表，并在开启 AI 时重新触发首轮说明话术。',
      )
    ) {
      return;
    }
    const timers = aiDebounceTimersRef.current;
    if (timers[userId]) {
      clearTimeout(timers[userId]);
      delete timers[userId];
    }
    suppressOutboundRef.current.add(userId);
    setConversations((prev) => {
      const next = { ...prev };
      delete next[userId];
      return next;
    });
    setUnread((prev) => {
      const next = { ...prev };
      delete next[userId];
      return next;
    });
    setAiPausedForUsers((prev) => {
      const next = { ...prev };
      delete next[userId];
      return next;
    });
    setActiveUserId((prev) => (prev === userId ? null : prev));
    console.log('[CS-Workbench] 已结束会话', { peerLen: userId.length });
  }, []);

  // 监听 WebSocket 消息
  useEffect(() => {
    const conn = connectionStore.connection;
    if (!conn) return;

    conn.setMessageHandler((proto: Proto) => {
      if (proto.op === Op.SendMsgReply) {
        if (proto.body && proto.body.length > 0) {
          try {
            const ack = parseCommonAck(proto.body);
            console.log(`[CS-WS] SendMsgReply: mid=${ack.mid}, seq=${proto.seq}, ack=${proto.ack}`);
          } catch {
            console.warn('[CS-WS] SendMsgReply: 解析失败', proto);
          }
        } else {
          console.log(`[CS-WS] SendMsgReply: seq=${proto.seq}, ack=${proto.ack} (无body)`);
        }
        return;
      }

      if (proto.op === Op.ReceiveMsg && proto.body) {
        // 先快速解析 from 地址，跳过自回声消息避免无用的解密调用
        const quickParsed = tryParseMessage(proto.body);
        if (quickParsed && quickParsed.from === agentAddr) {
          scheduleDelayedReceiveMsgAck(conn, proto.seq, effectiveReceiveAckDelayMs(loginResult));
          return;
        }

        (async () => {
          const parsed = await tryParseAndDecryptMessage(proto.body!);
          if (parsed) {
            suppressOutboundRef.current.delete(parsed.from);
            const msg: ChatMessage = {
              id: `msg-${Date.now()}-${msgIdCounter.current++}`,
              from: parsed.from,
              to: parsed.to,
              content: parsed.content,
              timestamp: Date.now(),
              isSelf: false,
            };
            addMessage(parsed.from, msg);

            setUnread((prev) => {
              if (parsed.from === activeUserId) return prev;
              return { ...prev, [parsed.from]: (prev[parsed.from] || 0) + 1 };
            });

            if (
              aiAutoReplyRef.current &&
              !aiPausedRef.current[parsed.from] &&
              parsed.content.trim() &&
              parsed.from !== agentAddr
            ) {
              scheduleAiAutoReply(parsed.from);
            }
          }
        })();
        scheduleDelayedReceiveMsgAck(conn, proto.seq, effectiveReceiveAckDelayMs(loginResult));
      }
    });
  }, [addMessage, activeUserId, agentAddr, scheduleAiAutoReply]);

  // 自动滚到底部
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [conversations, activeUserId]);

  // 选中对话时清除未读
  useEffect(() => {
    if (activeUserId) {
      setUnread((prev) => ({ ...prev, [activeUserId]: 0 }));
    }
  }, [activeUserId]);

  const handleSend = () => {
    if (!inputText.trim() || !activeUserId) return;
    if (connState !== 'connected') return;
    const content = inputText.trim();
    setInputText('');
    sendTextToPeer(activeUserId, content);
  };

  const handleLogout = async () => {
    try {
      await updateAgentStatus(0);
    } catch {
      // ignore
    }
    clearPersistedSession();
    connectionStore.connection?.disconnect();
    connectionStore.connection = null;
    connectionStore.loginResult = null;
    connectionStore.connectParams = null;
    onLogout();
  };

  const currentMessages = activeUserId ? conversations[activeUserId] || [] : [];
  const totalUnread = Object.values(unread).reduce((sum, n) => sum + n, 0);

  const handleChainPing = async () => {
    setChainBusy(true);
    setChainMsg('');
    try {
      const r = await chainPing(chainRpcInput.trim());
      setChainPingHeight(r.height);
      setChainMsg(`链节点正常，当前高度 ${r.height}`);
    } catch (e) {
      setChainPingHeight(null);
      setChainMsg(e instanceof Error ? e.message : String(e));
    } finally {
      setChainBusy(false);
    }
  };

  const handleBindComet = async () => {
    const comet = connectionStore.connectParams?.cometUrl;
    if (!comet) {
      setChainMsg('未找到登录时的 Comet 地址');
      return;
    }
    setChainBusy(true);
    setChainMsg('');
    try {
      const r = await bindCometOnChain(comet, chainRpcInput.trim());
      setChainMsg(`已提交链上绑定，tx: ${r.txHash}`);
    } catch (e) {
      setChainMsg(e instanceof Error ? e.message : String(e));
    } finally {
      setChainBusy(false);
    }
  };

  return (
    <div style={styles.layout}>
      {/* 左侧边栏 */}
      <div style={styles.sidebar}>
        <div style={styles.sidebarHeader}>
          <div>
            <div style={styles.agentName}>{agentName}</div>
            <div style={{ ...styles.agentStatus, display: 'flex', alignItems: 'center', gap: 6 }}>
              <span style={{
                display: 'inline-block', width: 8, height: 8, borderRadius: '50%',
                background: stateLabels[connState].color,
              }} />
              {stateLabels[connState].text}
              {connDetail && connState !== 'connected' && <span style={{ fontSize: 10, opacity: 0.7 }}>({connDetail})</span>}
            </div>
          </div>
          <button style={styles.logoutBtn} onClick={handleLogout}>
            下线
          </button>
        </div>

        <div style={styles.aiPanel}>
          <div style={styles.aiPanelTitle}>AI 自动客服（DeepSeek）</div>
          <label style={styles.aiToggleRow}>
            <input
              type="checkbox"
              checked={aiAutoReply}
              onChange={(e) => setAiAutoReply(e.target.checked)}
            />
            <span>收到用户消息后自动生成回复并发送</span>
          </label>
          <button
            type="button"
            style={styles.aiExpandBtn}
            onClick={() => setShowAiPolicyEditor((v) => !v)}
          >
            {showAiPolicyEditor ? '收起话术模版 / 转人工设置' : '展开话术模版 / 转人工设置'}
          </button>
          {showAiPolicyEditor && (
            <div style={styles.aiPolicyForm}>
              <label style={styles.aiLabel}>规范与话术要点</label>
              <div style={styles.aiFieldHint}>
                写入业务范围、标准应答结构、禁止承诺等；将追加给模型作为「业务规范」。
              </div>
              <textarea
                style={styles.aiTextarea}
                value={aiPolicy.playbook}
                onChange={(e) =>
                  setAiPolicy((p) => ({ ...p, playbook: e.target.value }))
                }
                placeholder={'示例：\n- 工作时间 9:00–18:00\n- 到账问题请先核对链上确认数\n- 勿承诺具体到账分钟数'}
                rows={7}
              />
              <label style={styles.aiLabel}>首轮主动发送 · 能力与事项说明</label>
              <div style={styles.aiFieldHint}>
                用户在本会话的<strong>第一条</strong>消息到达后，会先向对方发送本段内容（支持 Markdown），再调用模型回答对方具体问题。
                若清空并保存，则跳过首轮说明。
              </div>
              <textarea
                style={styles.aiTextarea}
                value={aiPolicy.proactiveIntroTemplate}
                onChange={(e) =>
                  setAiPolicy((p) => ({
                    ...p,
                    proactiveIntroTemplate: e.target.value,
                  }))
                }
                placeholder={DEFAULT_AI_POLICY.proactiveIntroTemplate}
                rows={10}
              />
              <label style={styles.aiLabel}>转人工固定话术</label>
              <div style={styles.aiFieldHint}>
                当模型判断需转人工时，须在回复中体现此话术含义；客户看到的正文不含技术标记。
              </div>
              <textarea
                style={styles.aiTextareaSm}
                value={aiPolicy.handoffClosingTemplate}
                onChange={(e) =>
                  setAiPolicy((p) => ({
                    ...p,
                    handoffClosingTemplate: e.target.value,
                  }))
                }
                rows={3}
              />
              <button
                type="button"
                style={styles.aiSaveBtn}
                onClick={() => {
                  if (!agentAddr) return;
                  saveAiPolicy(agentAddr, aiPolicy);
                }}
              >
                保存话术配置
              </button>
            </div>
          )}
          <div style={styles.aiHint}>
            密钥写入项目目录下 <code style={{ fontSize: 11 }}>.env.local</code> 的{' '}
            <code style={{ fontSize: 11 }}>DEEPSEEK_API_KEY</code>（可参考{' '}
            <code style={{ fontSize: 11 }}>.env.example</code>）。勿提交密钥；生产环境请用网关反向代理携带 Authorization。
          </div>
        </div>

        <div style={styles.chainPanel}>
          <div style={styles.chainPanelTitle}>链上与 Comet 绑定</div>
          <div style={styles.chainHint}>
            与 ChatPro 相同：链上 <code style={{ fontSize: 11 }}>chatServers</code> 需指向当前 Comet；CS 服务需配置代扣私钥。
          </div>
          <input
            style={styles.chainInput}
            value={chainRpcInput}
            onChange={(e) => setChainRpcInput(e.target.value)}
            placeholder="http://localhost:8901"
            disabled={chainBusy}
          />
          <div style={styles.chainActions}>
            <button type="button" style={styles.chainBtn} onClick={handleChainPing} disabled={chainBusy}>
              测试链节点
            </button>
            <button type="button" style={styles.chainBtnPrimary} onClick={handleBindComet} disabled={chainBusy}>
              绑定 Comet 到链上
            </button>
          </div>
          {chainPingHeight !== null && (
            <div style={styles.chainMeta}>最近探测高度: {chainPingHeight}</div>
          )}
          {chainMsg && <div style={styles.chainMsg}>{chainMsg}</div>}
        </div>

        <div style={styles.sidebarSubtitle}>
          对话 ({sortedUserIds.length})
          {totalUnread > 0 && <span style={styles.badge}>{totalUnread}</span>}
        </div>

        <div style={styles.sessionList}>
          {sortedUserIds.length === 0 && (
            <p style={styles.emptyText}>等待用户发起咨询...</p>
          )}
          {sortedUserIds.map((userId) => {
            const msgs = conversations[userId];
            const lastMsg = msgs[msgs.length - 1];
            const unreadCount = unread[userId] || 0;
            const humanHold = !!aiPausedForUsers[userId];
            return (
              <div
                key={userId}
                style={{
                  ...styles.sessionItem,
                  background: userId === activeUserId ? '#e8f4fd' : '#fff',
                  borderLeft: userId === activeUserId ? '3px solid #3498db' : '3px solid transparent',
                }}
                onClick={() => setActiveUserId(userId)}
              >
                <div style={styles.sessionAvatar}>
                  {userId.slice(0, 4)}
                </div>
                <div style={styles.sessionInfo}>
                  <div style={styles.sessionTitleRow}>
                    <span style={styles.sessionUserText}>
                      {userId.length > 16
                        ? `${userId.slice(0, 8)}...${userId.slice(-6)}`
                        : userId}
                    </span>
                    {humanHold && <span style={styles.humanChip}>人工</span>}
                  </div>
                  <div style={styles.lastMsg}>
                    {lastMsg ? (lastMsg.isSelf ? '我: ' : '') + truncate(lastMsg.content, 20) : ''}
                  </div>
                </div>
                <div style={styles.sessionRight}>
                  <div style={styles.sessionTime}>
                    {lastMsg ? new Date(lastMsg.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : ''}
                  </div>
                  {unreadCount > 0 && (
                    <span style={styles.unreadBadge}>{unreadCount}</span>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* 右侧聊天区域 */}
      <div style={styles.chatArea}>
        {!activeUserId ? (
          <div style={styles.placeholder}>
            {loginResult && showLoginResult ? (
              <LoginResultPanel result={loginResult} onClose={() => setShowLoginResult(false)} />
            ) : (
              <>
                <div style={styles.placeholderIcon}>💬</div>
                <p style={styles.placeholderText}>
                  用户通过客服地址发起私聊后<br />对话将自动出现在左侧列表
                </p>
                {loginResult && !showLoginResult && (
                  <button
                    style={styles.showResultBtn}
                    onClick={() => setShowLoginResult(true)}
                  >
                    查看登录详情
                  </button>
                )}
              </>
            )}
          </div>
        ) : (
          <>
            <div style={styles.chatHeader}>
              <div style={styles.chatHeaderRow}>
                <span style={styles.chatTitle}>
                  {activeUserId.length > 24
                    ? `${activeUserId.slice(0, 12)}...${activeUserId.slice(-10)}`
                    : activeUserId}
                </span>
                <div style={styles.chatHeaderActions}>
                  {aiAutoReply && !aiPausedForUsers[activeUserId] && (
                    <button
                      type="button"
                      style={styles.pauseAiBtn}
                      onClick={() =>
                        setAiPausedForUsers((prev) => ({
                          ...prev,
                          [activeUserId]: true,
                        }))
                      }
                    >
                      暂停 AI（人工接管）
                    </button>
                  )}
                  {aiPausedForUsers[activeUserId] && (
                    <button
                      type="button"
                      style={styles.resumeAiBtn}
                      onClick={() =>
                        setAiPausedForUsers((prev) => {
                          const next = { ...prev };
                          delete next[activeUserId];
                          return next;
                        })
                      }
                    >
                      恢复 AI
                    </button>
                  )}
                  <button
                    type="button"
                    style={styles.closeSessionBtn}
                    onClick={() => handleCloseConversation(activeUserId)}
                  >
                    结束会话
                  </button>
                </div>
              </div>
              {aiPausedForUsers[activeUserId] && (
                <div style={styles.humanBanner}>
                  本会话已暂停 AI 自动回复：请人工处理；处理完后可点「恢复 AI」继续由模型辅助。
                </div>
              )}
            </div>

            <div style={styles.messageList}>
              {currentMessages.map((msg) => (
                <div
                  key={msg.id}
                  style={{
                    ...styles.msgRow,
                    justifyContent: msg.isSelf ? 'flex-end' : 'flex-start',
                  }}
                >
                  <div
                    style={{
                      ...styles.msgBubble,
                      background: msg.isSelf ? 'var(--msg-self)' : 'var(--msg-other)',
                      borderBottomRightRadius: msg.isSelf ? 2 : 'var(--radius)',
                      borderBottomLeftRadius: msg.isSelf ? 'var(--radius)' : 2,
                    }}
                  >
                    <ChatMarkdown text={msg.content} isSelf={msg.isSelf} />
                    <span style={styles.msgTime}>
                      {new Date(msg.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                    </span>
                  </div>
                </div>
              ))}
              <div ref={messagesEndRef} />
            </div>

            {connState !== 'connected' && (
              <div style={styles.reconnectBanner}>
                <span style={styles.reconnectDot} />
                {stateLabels[connState].text}
                {connDetail && <span style={{ fontSize: 11, opacity: 0.7, marginLeft: 6 }}>({connDetail})</span>}
              </div>
            )}
            <div style={styles.inputBar}>
              <input
                style={styles.chatInput}
                value={inputText}
                onChange={(e) => setInputText(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleSend()}
                placeholder={connState === 'connected' ? '输入消息, 按 Enter 发送...' : '连接断开，等待重连...'}
                disabled={connState !== 'connected'}
              />
              <button
                style={{ ...styles.sendBtn, opacity: connState === 'connected' ? 1 : 0.5 }}
                onClick={handleSend}
                disabled={connState !== 'connected'}
              >
                发送
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

function LoginResultPanel({ result, onClose }: { result: LoginResult; onClose: () => void }) {
  const sections = [
    {
      title: '1. CS 登录',
      rows: [
        { label: 'Agent ID', value: result.csLogin.agent.agentId },
        { label: 'Agent Name', value: result.csLogin.agent.name },
        { label: 'Agent Avatar', value: result.csLogin.agent.avatar || '(空)' },
        { label: 'Agent Status', value: String(result.csLogin.agent.status) },
        { label: 'Session Token', value: result.csLogin.sessionToken },
      ],
    },
    {
      title: '2. FZM-SIGNATURE',
      rows: [
        { label: 'Signature 长度', value: String(result.fzmSignature.length) },
        { label: 'Signature', value: result.fzmSignature.signature },
      ],
    },
    {
      title: '3. Comet WebSocket (连接参数)',
      rows: [
        { label: 'WebSocket URL', value: result.cometConnect.wsUrl },
        { label: 'AppId', value: result.cometConnect.appId },
        { label: 'Device UUID', value: result.cometConnect.deviceUUID },
        { label: 'Device Name', value: result.cometConnect.deviceName },
        { label: 'Login Ext 长度', value: String(result.cometConnect.extLength) },
      ],
    },
    {
      title: '3.1 Comet AuthReply (服务器返回)',
      rows: [
        { label: 'Proto.Ver', value: String(result.cometConnect.authReply.ver) },
        { label: 'Proto.Op', value: `${result.cometConnect.authReply.op} (${result.cometConnect.authReply.opName})` },
        { label: 'Proto.Seq', value: String(result.cometConnect.authReply.seq) },
        { label: 'Proto.Ack', value: String(result.cometConnect.authReply.ack) },
        { label: 'Body 长度', value: `${result.cometConnect.authReply.bodyLength} 字节` },
        { label: 'Body Hex', value: result.cometConnect.authReply.bodyHex },
        { label: 'Body 文本', value: result.cometConnect.authReply.bodyText },
        { label: '认证结果', value: result.cometConnect.authReply.success ? '成功 (Body 为空)' : '失败 (Body 含错误信息)' },
      ],
    },
    {
      title: '4. 客服状态更新',
      rows: [
        { label: '是否成功', value: result.agentStatusUpdate.success ? '成功' : '失败' },
        { label: '详情', value: result.agentStatusUpdate.detail },
      ],
    },
  ];

  return (
    <div style={resultStyles.container}>
      <div style={resultStyles.header}>
        <span style={resultStyles.headerTitle}>登录结果详情</span>
        <span style={resultStyles.headerTime}>{result.loginTime}</span>
        <button style={resultStyles.closeBtn} onClick={onClose}>收起</button>
      </div>
      <div style={resultStyles.body}>
        {sections.map((sec) => (
          <div key={sec.title} style={resultStyles.section}>
            <div style={resultStyles.sectionTitle}>{sec.title}</div>
            {sec.rows.map((row) => (
              <div key={row.label} style={resultStyles.row}>
                <span style={resultStyles.label}>{row.label}</span>
                <span style={resultStyles.value}>{row.value}</span>
              </div>
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}

const resultStyles: Record<string, React.CSSProperties> = {
  container: {
    width: '100%',
    maxWidth: 700,
    maxHeight: '80vh',
    display: 'flex',
    flexDirection: 'column',
    background: '#fff',
    borderRadius: 12,
    boxShadow: '0 4px 24px rgba(0,0,0,0.08)',
    border: '1px solid var(--border)',
    overflow: 'hidden',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
    padding: '14px 20px',
    background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
    color: '#fff',
    flexShrink: 0,
  },
  headerTitle: {
    fontWeight: 700,
    fontSize: 15,
    flex: 1,
  },
  headerTime: {
    fontSize: 12,
    opacity: 0.8,
  },
  closeBtn: {
    background: 'rgba(255,255,255,0.2)',
    color: '#fff',
    fontSize: 12,
    padding: '4px 12px',
    border: '1px solid rgba(255,255,255,0.4)',
    borderRadius: 4,
    cursor: 'pointer',
  },
  body: {
    padding: '16px 20px',
    overflowY: 'auto',
    display: 'flex',
    flexDirection: 'column',
    gap: 16,
  },
  section: {
    background: '#f8f9fa',
    borderRadius: 8,
    padding: '12px 16px',
    border: '1px solid #e9ecef',
  },
  sectionTitle: {
    fontWeight: 700,
    fontSize: 13,
    color: '#667eea',
    marginBottom: 8,
    paddingBottom: 6,
    borderBottom: '1px solid #e9ecef',
  },
  row: {
    display: 'flex',
    padding: '4px 0',
    fontSize: 13,
    gap: 12,
    alignItems: 'flex-start',
  },
  label: {
    color: '#555',
    fontWeight: 600,
    flexShrink: 0,
    minWidth: 120,
  },
  value: {
    color: '#333',
    wordBreak: 'break-all',
    fontFamily: 'monospace',
    fontSize: 12,
    lineHeight: 1.5,
  },
};

function truncate(s: string, max: number): string {
  return s.length > max ? s.slice(0, max) + '...' : s;
}

function parseCommonAck(raw: Uint8Array): { mid: number; datetime: number } {
  const biz = decodeBizProto(raw);
  if (biz.eventType === EventType.CommonAck && biz.body) {
    return decodeCommonAck(biz.body);
  }
  return decodeCommonAck(raw);
}

function tryParseMessage(
  raw: Uint8Array
): { from: string; to: string; content: string } | null {
  const parsed = parseReceivedMessage(raw);
  if (!parsed) return null;

  const { common, textContent } = parsed;
  return {
    from: common.from || 'unknown',
    to: common.target || '',
    content: textContent ?? `[${MsgType[common.msgType] ?? '消息'}]`,
  };
}

async function tryParseAndDecryptMessage(
  raw: Uint8Array
): Promise<{ from: string; to: string; content: string } | null> {
  try {
    const biz = decodeBizProto(raw);
    if (biz.eventType !== EventType.Common || !biz.body) return null;

    const common = decodeCommon(biz.body);
    const senderAddr = common.from || 'unknown';
    const targetAddr = common.target || '';

    if (common.msgType === MsgType.Text && common.msg && common.msg.length > 0) {
      // 先尝试直接解码（明文消息）
      let textContent: string | null = null;
      try {
        const textMsg = decodeTextMsg(common.msg);
        if (textMsg.content) {
          textContent = textMsg.content;
        }
      } catch {
        // 明文解码失败
      }

      // 如果明文解码失败或内容为空/乱码，尝试解密
      if (!textContent || !isPrintable(textContent)) {
        try {
          const decResp = await decryptMsg(senderAddr, uint8ToBase64(common.msg));
          const decrypted = base64ToUint8(decResp.data);
          const textMsg = decodeTextMsg(decrypted);
          if (textMsg.content) {
            console.log('[CS-WS] 消息已解密', { from: senderAddr, contentLen: textMsg.content.length });
            textContent = textMsg.content;
          }
        } catch (decErr) {
          console.warn('[CS-WS] 解密失败，使用原始解析结果', decErr);
        }
      }

      return {
        from: senderAddr,
        to: targetAddr,
        content: textContent ?? `[${MsgType[common.msgType] ?? '消息'}]`,
      };
    }

    return {
      from: senderAddr,
      to: targetAddr,
      content: `[${MsgType[common.msgType] ?? '未知类型'}]`,
    };
  } catch {
    return tryParseMessage(raw);
  }
}

function isPrintable(s: string): boolean {
  if (!s) return false;
  for (let i = 0; i < Math.min(s.length, 20); i++) {
    const code = s.charCodeAt(i);
    if (code < 0x20 && code !== 0x0a && code !== 0x0d && code !== 0x09) return false;
  }
  return true;
}

const styles: Record<string, React.CSSProperties> = {
  layout: {
    display: 'flex',
    height: '100%',
    overflow: 'hidden',
  },
  sidebar: {
    width: 320,
    background: '#fff',
    borderRight: '1px solid var(--border)',
    display: 'flex',
    flexDirection: 'column',
    flexShrink: 0,
  },
  sidebarHeader: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: '16px 20px',
    borderBottom: '1px solid var(--border)',
    background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
    color: '#fff',
  },
  agentName: {
    fontSize: 16,
    fontWeight: 700,
  },
  agentStatus: {
    fontSize: 12,
    opacity: 0.8,
    marginTop: 2,
  },
  logoutBtn: {
    background: 'rgba(255,255,255,0.2)',
    color: '#fff',
    fontSize: 13,
    padding: '6px 14px',
    border: '1px solid rgba(255,255,255,0.4)',
    borderRadius: 6,
  },
  aiPanel: {
    padding: '10px 16px',
    borderBottom: '1px solid var(--border)',
    background: '#f6f8fc',
    flexShrink: 0,
  },
  aiPanelTitle: {
    fontSize: 12,
    fontWeight: 700,
    color: '#333',
    marginBottom: 8,
  },
  aiToggleRow: {
    display: 'flex',
    alignItems: 'flex-start',
    gap: 10,
    fontSize: 13,
    color: '#333',
    cursor: 'pointer',
    lineHeight: 1.45,
  },
  aiExpandBtn: {
    marginTop: 10,
    width: '100%',
    padding: '8px 10px',
    fontSize: 12,
    borderRadius: 6,
    border: '1px solid #c5cae8',
    background: '#fff',
    color: '#5c6bc0',
    fontWeight: 600,
    cursor: 'pointer',
  },
  aiPolicyForm: {
    marginTop: 12,
    paddingTop: 12,
    borderTop: '1px solid #e8eaf2',
  },
  aiLabel: {
    display: 'block',
    fontSize: 12,
    fontWeight: 700,
    color: '#333',
    marginBottom: 4,
    marginTop: 10,
  },
  aiFieldHint: {
    fontSize: 11,
    color: 'var(--text-secondary)',
    lineHeight: 1.4,
    marginBottom: 6,
  },
  aiTextarea: {
    width: '100%',
    minHeight: 120,
    padding: '8px 10px',
    fontSize: 12,
    border: '1px solid var(--border)',
    borderRadius: 6,
    boxSizing: 'border-box',
    fontFamily: 'inherit',
    lineHeight: 1.45,
    resize: 'vertical' as const,
  },
  aiTextareaSm: {
    width: '100%',
    minHeight: 56,
    padding: '8px 10px',
    fontSize: 12,
    border: '1px solid var(--border)',
    borderRadius: 6,
    boxSizing: 'border-box',
    fontFamily: 'inherit',
    lineHeight: 1.45,
    resize: 'vertical' as const,
  },
  aiSaveBtn: {
    marginTop: 12,
    padding: '8px 16px',
    fontSize: 13,
    fontWeight: 600,
    borderRadius: 6,
    border: 'none',
    background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
    color: '#fff',
    cursor: 'pointer',
  },
  aiHint: {
    marginTop: 8,
    fontSize: 11,
    color: 'var(--text-secondary)',
    lineHeight: 1.45,
  },
  chainPanel: {
    padding: '12px 16px',
    borderBottom: '1px solid var(--border)',
    background: '#fafbfc',
    flexShrink: 0,
  },
  chainPanelTitle: {
    fontSize: 12,
    fontWeight: 700,
    color: '#333',
    marginBottom: 6,
  },
  chainHint: {
    fontSize: 11,
    color: 'var(--text-secondary)',
    lineHeight: 1.45,
    marginBottom: 8,
  },
  chainInput: {
    width: '100%',
    padding: '8px 10px',
    fontSize: 12,
    border: '1px solid var(--border)',
    borderRadius: 6,
    marginBottom: 8,
    boxSizing: 'border-box',
  },
  chainActions: {
    display: 'flex',
    gap: 8,
    flexWrap: 'wrap',
  },
  chainBtn: {
    flex: 1,
    minWidth: 100,
    padding: '6px 10px',
    fontSize: 12,
    borderRadius: 6,
    border: '1px solid #dcdfe6',
    background: '#fff',
    cursor: 'pointer',
  },
  chainBtnPrimary: {
    flex: 1,
    minWidth: 100,
    padding: '6px 10px',
    fontSize: 12,
    borderRadius: 6,
    border: 'none',
    background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
    color: '#fff',
    cursor: 'pointer',
  },
  chainMeta: {
    fontSize: 11,
    color: '#27ae60',
    marginTop: 6,
  },
  chainMsg: {
    fontSize: 11,
    color: '#555',
    marginTop: 6,
    wordBreak: 'break-all',
  },
  sidebarSubtitle: {
    padding: '10px 20px',
    fontSize: 13,
    fontWeight: 600,
    color: 'var(--text-secondary)',
    borderBottom: '1px solid var(--border)',
    display: 'flex',
    alignItems: 'center',
    gap: 8,
  },
  badge: {
    background: 'var(--danger)',
    color: '#fff',
    borderRadius: 10,
    padding: '1px 7px',
    fontSize: 11,
    fontWeight: 700,
  },
  sessionList: {
    flex: 1,
    overflowY: 'auto',
  },
  emptyText: {
    textAlign: 'center',
    color: 'var(--text-secondary)',
    padding: 40,
    fontSize: 14,
    lineHeight: 1.8,
  },
  sessionItem: {
    display: 'flex',
    alignItems: 'center',
    padding: '12px 16px',
    cursor: 'pointer',
    transition: 'background 0.15s',
    gap: 12,
  },
  sessionAvatar: {
    width: 40,
    height: 40,
    borderRadius: '50%',
    background: 'linear-gradient(135deg, #667eea, #764ba2)',
    color: '#fff',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: 12,
    fontWeight: 600,
    flexShrink: 0,
  },
  sessionInfo: {
    flex: 1,
    minWidth: 0,
  },
  sessionTitleRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    minWidth: 0,
  },
  sessionUserText: {
    fontSize: 14,
    fontWeight: 500,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    flex: 1,
    minWidth: 0,
  },
  humanChip: {
    flexShrink: 0,
    fontSize: 10,
    fontWeight: 700,
    padding: '2px 6px',
    borderRadius: 4,
    background: '#fff3cd',
    color: '#856404',
  },
  lastMsg: {
    fontSize: 12,
    color: 'var(--text-secondary)',
    marginTop: 2,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  sessionRight: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'flex-end',
    gap: 4,
    flexShrink: 0,
  },
  sessionTime: {
    fontSize: 11,
    color: 'var(--text-secondary)',
  },
  unreadBadge: {
    background: 'var(--danger)',
    color: '#fff',
    borderRadius: 10,
    padding: '1px 6px',
    fontSize: 11,
    fontWeight: 700,
    minWidth: 18,
    textAlign: 'center' as const,
  },
  chatArea: {
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
    background: 'var(--bg-primary)',
  },
  placeholder: {
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
    justifyContent: 'center',
    alignItems: 'center',
    gap: 16,
  },
  placeholderIcon: {
    fontSize: 64,
  },
  placeholderText: {
    color: 'var(--text-secondary)',
    fontSize: 15,
    textAlign: 'center' as const,
    lineHeight: 1.8,
  },
  chatHeader: {
    padding: '12px 24px',
    background: '#fff',
    borderBottom: '1px solid var(--border)',
    fontSize: 15,
    display: 'flex',
    flexDirection: 'column' as const,
    gap: 10,
  },
  chatHeaderRow: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 12,
    fontWeight: 600,
  },
  chatHeaderActions: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    flexShrink: 0,
  },
  pauseAiBtn: {
    padding: '6px 12px',
    fontSize: 12,
    borderRadius: 6,
    border: '1px solid #f0ad4e',
    background: '#fffbf0',
    color: '#856404',
    cursor: 'pointer',
    fontWeight: 600,
  },
  resumeAiBtn: {
    padding: '6px 12px',
    fontSize: 12,
    borderRadius: 6,
    border: 'none',
    background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
    color: '#fff',
    cursor: 'pointer',
    fontWeight: 600,
  },
  closeSessionBtn: {
    padding: '6px 12px',
    fontSize: 12,
    borderRadius: 6,
    border: '1px solid #e74c3c',
    background: '#fff',
    color: '#c0392b',
    cursor: 'pointer',
    fontWeight: 600,
  },
  humanBanner: {
    fontSize: 12,
    fontWeight: 500,
    color: '#856404',
    background: '#fffbf0',
    border: '1px solid #ffe69c',
    borderRadius: 8,
    padding: '10px 12px',
    lineHeight: 1.45,
  },
  chatTitle: {
    flex: 1,
    minWidth: 0,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap' as const,
  },
  messageList: {
    flex: 1,
    overflowY: 'auto',
    padding: '16px 24px',
    display: 'flex',
    flexDirection: 'column',
    gap: 8,
  },
  msgRow: {
    display: 'flex',
  },
  msgBubble: {
    maxWidth: '65%',
    padding: '10px 14px',
    borderRadius: 'var(--radius)',
    fontSize: 14,
    lineHeight: 1.5,
    boxShadow: '0 1px 2px rgba(0,0,0,0.05)',
    wordBreak: 'break-word',
  },
  msgTime: {
    display: 'block',
    fontSize: 11,
    color: 'var(--text-secondary)',
    marginTop: 4,
    textAlign: 'right' as const,
  },
  inputBar: {
    display: 'flex',
    gap: 12,
    padding: '12px 24px',
    background: '#fff',
    borderTop: '1px solid var(--border)',
  },
  chatInput: {
    flex: 1,
    padding: '10px 14px',
    fontSize: 14,
    border: '1px solid var(--border)',
    borderRadius: 8,
  },
  sendBtn: {
    padding: '10px 24px',
    background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
    color: '#fff',
    fontWeight: 600,
    fontSize: 14,
    borderRadius: 8,
    border: 'none',
  },
  showResultBtn: {
    marginTop: 12,
    padding: '8px 20px',
    background: 'transparent',
    color: '#667eea',
    fontSize: 13,
    fontWeight: 500,
    border: '1px solid #667eea',
    borderRadius: 6,
    cursor: 'pointer',
  },
  reconnectBanner: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 8,
    padding: '8px 24px',
    background: '#fff3cd',
    color: '#856404',
    fontSize: 13,
    fontWeight: 500,
    borderTop: '1px solid #ffc107',
  },
  reconnectDot: {
    display: 'inline-block',
    width: 8,
    height: 8,
    borderRadius: '50%',
    background: '#f39c12',
    animation: 'pulse 1.5s infinite',
  },
};
