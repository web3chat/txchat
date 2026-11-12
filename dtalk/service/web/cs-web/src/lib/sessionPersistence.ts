import type { CSAgentPublic } from './api';
import { getAgentInfo, refreshFZMToken, setSessionToken, updateAgentStatus } from './api';
import { Op, Proto } from './protocol';
import {
  AuthReplyInfo,
  ConnectParams,
  connectionStore,
  LoginResult,
} from '../store';
import { encodeLoginExt, IMConnection } from './ws';

/** 与聊天记录 key 区分；仅存会话凭证与连接参数（不含密码） */
export const SESSION_STORAGE_VERSION = 1 as const;
const LS_SESSION = 'cs_web_session_v1';

export interface PersistedSession {
  version: typeof SESSION_STORAGE_VERSION;
  sessionToken: string;
  connectParams: ConnectParams;
  /** 顶栏展示 */
  agentName: string;
  agent: CSAgentPublic;
  /** 与 CS `cometReceiveAckDelayMillis` 一致；旧本地会话可能没有，恢复时会请求 agent/info 补全 */
  cometReceiveAckDelayMillis?: number;
}

/** 恢复/进入工作台时必须成功，否则 DB 仍为离线，CS 侧离线 AI 会继续代答。 */
async function updateAgentOnlineWithRetry(): Promise<void> {
  const attempts = 3;
  const delayMs = 500;
  let last: unknown;
  for (let i = 0; i < attempts; i++) {
    try {
      await updateAgentStatus(1);
      return;
    } catch (e) {
      last = e;
      if (i < attempts - 1) {
        await new Promise((r) => setTimeout(r, delayMs));
      }
    }
  }
  throw last;
}

function authReplyFromProto(authReplyProto: Proto): AuthReplyInfo {
  return {
    ver: authReplyProto.ver,
    op: authReplyProto.op,
    opName: Op[authReplyProto.op] || `UNKNOWN(${authReplyProto.op})`,
    seq: authReplyProto.seq,
    ack: authReplyProto.ack,
    bodyLength: authReplyProto.body?.length ?? 0,
    bodyHex: authReplyProto.body
      ? Array.from(authReplyProto.body.slice(0, 64))
          .map((b) => b.toString(16).padStart(2, '0'))
          .join(' ')
      : '(空)',
    bodyText: authReplyProto.body
      ? new TextDecoder().decode(authReplyProto.body)
      : '(空 - 认证成功)',
    success: !authReplyProto.body || authReplyProto.body.length === 0,
  };
}

export function persistSession(session: PersistedSession): void {
  if (typeof window === 'undefined') return;
  try {
    localStorage.setItem(LS_SESSION, JSON.stringify(session));
  } catch (e) {
    console.error('[CS-Session] 写入本地会话失败', e);
  }
}

export function loadPersistedSession(): PersistedSession | null {
  if (typeof window === 'undefined') return null;
  try {
    const raw = localStorage.getItem(LS_SESSION);
    if (!raw) return null;
    const data = JSON.parse(raw) as PersistedSession;
    if (
      data.version !== SESSION_STORAGE_VERSION ||
      !data.sessionToken ||
      !data.connectParams?.cometUrl ||
      !data.connectParams?.appId ||
      !data.connectParams?.deviceUUID ||
      !data.agent?.agentId
    ) {
      return null;
    }
    return data;
  } catch (e) {
    console.warn('[CS-Session] 读取本地会话失败', e);
    return null;
  }
}

export function clearPersistedSession(): void {
  restoreSingleton = null;
  if (typeof window === 'undefined') return;
  try {
    localStorage.removeItem(LS_SESSION);
  } catch (e) {
    console.warn('[CS-Session] 清除本地会话失败', e);
  }
}

/** 用已保存的 HTTP session 换 FZM、重建 Comet 连接（刷新页面后调用） */
export async function restorePersistedCometSession(
  snap: PersistedSession,
): Promise<{ conn: IMConnection; loginResult: LoginResult }> {
  setSessionToken(snap.sessionToken);
  const { fzmSignature } = await refreshFZMToken();

  const ext = encodeLoginExt(
    snap.connectParams.deviceUUID,
    snap.connectParams.deviceName,
  );
  const cometUrl = snap.connectParams.cometUrl;
  const wsUrl = cometUrl.replace(/^http/, 'ws') + '/sub';

  const conn = new IMConnection(cometUrl);
  conn.setRefreshTokenHandler(async () => {
    console.log('[CS-Session] 重连: 刷新 FZM-SIGNATURE...');
    const { fzmSignature: newSig } = await refreshFZMToken();
    return newSig;
  });

  const authReplyProto = await conn.connect({
    appId: snap.connectParams.appId,
    token: fzmSignature,
    ext,
  });

  const authReply = authReplyFromProto(authReplyProto);

  let cometReceiveAckDelayMillis = snap.cometReceiveAckDelayMillis;
  if (cometReceiveAckDelayMillis == null || cometReceiveAckDelayMillis <= 0) {
    try {
      const info = await getAgentInfo();
      cometReceiveAckDelayMillis =
        info.cometReceiveAckDelayMillis != null && info.cometReceiveAckDelayMillis > 0
          ? info.cometReceiveAckDelayMillis
          : 280;
    } catch {
      cometReceiveAckDelayMillis = 280;
    }
  }

  let agentStatusUpdate: LoginResult['agentStatusUpdate'];
  try {
    await updateAgentOnlineWithRetry();
    agentStatusUpdate = { success: true, detail: '恢复会话' };
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    agentStatusUpdate = { success: false, detail: msg };
    console.error(
      '[CS-Session] 恢复后无法将客服标为在线（已重试），服务端仍可能按「离线」运行离线 AI。请检查网络与 POST /cs/agent/status。',
      e,
    );
  }

  const loginResult: LoginResult = {
    csLogin: {
      sessionToken: snap.sessionToken,
      agent: snap.agent,
    },
    fzmSignature: {
      signature: fzmSignature,
      length: fzmSignature.length,
    },
    cometConnect: {
      wsUrl,
      appId: snap.connectParams.appId,
      deviceUUID: snap.connectParams.deviceUUID,
      deviceName: snap.connectParams.deviceName,
      extLength: ext.length,
      authReply,
    },
    agentStatusUpdate,
    loginTime: new Date().toLocaleString(),
    cometReceiveAckDelayMillis,
  };

  return { conn, loginResult };
}

export type SessionRestoreOutcome = { agentName: string | null };

/** React Strict Mode 下 effect 会跑两次：共享同一 Promise，只恢复一条 Comet 连接 */
let restoreSingleton: Promise<SessionRestoreOutcome> | null = null;

export function getSessionRestoreOutcome(): Promise<SessionRestoreOutcome> {
  restoreSingleton ??= (async (): Promise<SessionRestoreOutcome> => {
    const snap = loadPersistedSession();
    if (!snap) {
      return { agentName: null };
    }
    try {
      const { conn, loginResult } = await restorePersistedCometSession(snap);
      connectionStore.connection = conn;
      connectionStore.connectParams = snap.connectParams;
      connectionStore.loginResult = loginResult;
      console.log('[CS-Session] 已从本地会话恢复', {
        agentIdLen: snap.agent.agentId.length,
        agentName: snap.agentName,
      });
      return { agentName: snap.agentName };
    } catch (e) {
      console.error('[CS-Session] 恢复会话失败，需重新登录', e);
      clearPersistedSession();
      connectionStore.connection = null;
      connectionStore.connectParams = null;
      connectionStore.loginResult = null;
      return { agentName: null };
    }
  })();
  return restoreSingleton;
}
