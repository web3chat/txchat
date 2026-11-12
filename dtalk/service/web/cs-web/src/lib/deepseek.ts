import type { ChatMessage } from '../store';

const TAG = '[CS-AI]';

/** 可根据业务在后续改为可配置 */
export const DEFAULT_CS_SYSTEM_PROMPT = [
  '你是「TxChat / ChatPro」体系的在线客服助手，使用简体中文回复。',
  '回答简洁、礼貌、专业；不清楚的业务不要随意承诺，可引导用户联系人工或查看官方说明。',
  '不要编造链上交易哈希、私钥、助记词；不要引导用户提供密码或私钥。',
  '说明步骤或参数时，可使用 Markdown（如分级标题、有序/无序列表、**粗体**、行内 `代码`、```围栏代码块```），便于用户阅读。',
].join('\n');

export type DeepSeekChatRole = 'system' | 'user' | 'assistant';

export interface DeepSeekChatMessage {
  role: DeepSeekChatRole;
  content: string;
}

/** 取最近若干条，映射为 Chat Completions 的 messages（含 system） */
export function historyToDeepseekMessages(
  history: ChatMessage[],
  options?: { maxTurns?: number; systemPrompt?: string },
): DeepSeekChatMessage[] {
  const maxTurns = options?.maxTurns ?? 40;
  const systemPrompt = options?.systemPrompt ?? DEFAULT_CS_SYSTEM_PROMPT;
  const slice = history.slice(-maxTurns);
  const out: DeepSeekChatMessage[] = [{ role: 'system', content: systemPrompt }];
  for (const m of slice) {
    const role: DeepSeekChatRole = m.isSelf ? 'assistant' : 'user';
    const content = (m.content || '').trim();
    if (!content) continue;
    out.push({ role, content });
  }
  return out;
}

interface ChatCompletionResponse {
  choices?: Array<{ message?: { content?: string | null } }>;
  error?: { message?: string };
}

/**
 * 浏览器请求同源 `/deepseek-proxy/v1/...`，由 Vite 开发服务器转发并附带服务端 DEEPSEEK_API_KEY。
 * 生产环境需在网关/Nginx 配置等价反向代理，切勿把密钥写入 import.meta.env.VITE_*。
 */
export async function requestDeepseekReply(
  history: ChatMessage[],
  options?: {
    signal?: AbortSignal;
    model?: string;
    /** 完整 system（覆盖默认客服人设），一般由 buildFullSystemPrompt 生成 */
    systemPrompt?: string;
    temperature?: number;
  },
): Promise<string> {
  const messages = historyToDeepseekMessages(history, {
    systemPrompt: options?.systemPrompt,
  });
  if (messages.length <= 1) {
    console.warn(`${TAG} 无有效对话内容，跳过请求`);
    return '';
  }

  const body = {
    model: options?.model ?? 'deepseek-chat',
    messages,
    temperature: options?.temperature ?? 0.5,
    max_tokens: 1024,
  };

  const res = await fetch('/deepseek-proxy/v1/chat/completions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
    signal: options?.signal,
  });

  const rawText = await res.text();
  if (!res.ok) {
    console.error(`${TAG} HTTP ${res.status}`, rawText.slice(0, 600));
    throw new Error(`DeepSeek API ${res.status}: ${rawText.slice(0, 240)}`);
  }

  let data: ChatCompletionResponse;
  try {
    data = JSON.parse(rawText) as ChatCompletionResponse;
  } catch (e) {
    console.error(`${TAG} JSON 解析失败`, rawText.slice(0, 400));
    throw e;
  }

  if (data.error?.message) {
    console.error(`${TAG} 接口错误`, data.error.message);
    throw new Error(data.error.message);
  }

  const content = data.choices?.[0]?.message?.content?.trim();
  if (!content) {
    console.error(`${TAG} 空 choices`, rawText.slice(0, 400));
    throw new Error('DeepSeek 返回空内容');
  }

  console.log(`${TAG} 回复长度=${content.length}`);
  return content;
}
