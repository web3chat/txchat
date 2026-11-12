import type { ChatMessage } from '../store';

export const STORAGE_VERSION = 1 as const;
const PREFIX = 'cs-web.chat.v1';

export interface PersistedChatState {
  version: typeof STORAGE_VERSION;
  conversations: Record<string, ChatMessage[]>;
  activeUserId: string | null;
}

function storageKey(agentId: string): string {
  return `${PREFIX}:${agentId}`;
}

/** 从已有消息 id（msg-<ts>-<n>）推导下一个计数种子，避免刷新后与旧 id 冲突 */
export function nextMsgCounterSeed(conversations: Record<string, ChatMessage[]>): number {
  let max = -1;
  for (const list of Object.values(conversations)) {
    for (const m of list) {
      const match = /^msg-\d+-(\d+)$/.exec(m.id);
      if (match) {
        const n = Number(match[1]);
        if (n > max) max = n;
      }
    }
  }
  return max + 1;
}

export function loadChatState(agentId: string): PersistedChatState | null {
  if (!agentId || typeof window === 'undefined') return null;
  try {
    const raw = localStorage.getItem(storageKey(agentId));
    if (!raw) return null;
    const data = JSON.parse(raw) as PersistedChatState;
    if (data.version !== STORAGE_VERSION || !data.conversations || typeof data.conversations !== 'object') {
      return null;
    }
    const active =
      typeof data.activeUserId === 'string' && data.conversations[data.activeUserId]
        ? data.activeUserId
        : null;
    return {
      version: STORAGE_VERSION,
      conversations: data.conversations,
      activeUserId: active,
    };
  } catch (e) {
    console.warn('[CS-WS] 读取本地聊天记录失败', { agentIdLen: agentId.length, err: e });
    return null;
  }
}

export function saveChatState(agentId: string, state: PersistedChatState): void {
  if (!agentId || typeof window === 'undefined') return;
  try {
    const payload = JSON.stringify(state);
    if (payload.length > 4_500_000) {
      console.warn('[CS-WS] 聊天记录体积过大，跳过写入本地', {
        agentIdLen: agentId.length,
        approxBytes: payload.length,
      });
      return;
    }
    localStorage.setItem(storageKey(agentId), payload);
  } catch (e) {
    console.error('[CS-WS] 保存本地聊天记录失败', { agentIdLen: agentId.length, err: e });
  }
}
