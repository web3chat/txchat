/**
 * Gateway HTTP API 封装
 */

const BASE_URL = '/api/v1';
const TAG = '[CS-API]';

interface ApiResponse<T = unknown> {
  result: number;
  message?: string;
  data?: T;
}

let sessionToken = '';

export function setSessionToken(token: string) {
  console.log(`${TAG} setSessionToken: ${token.substring(0, 20)}... (len=${token.length})`);
  sessionToken = token;
}

export function getSessionToken() {
  return sessionToken;
}

/** 解析网关统一封装 JSON；兼容异常响应、BOM、首尾杂质 */
function parseGatewayJson<T>(text: string, url: string, httpStatus: number): ApiResponse<T> {
  const raw = text.replace(/^\uFEFF/, '').trim();
  if (!raw) {
    throw new Error(`接口返回空响应 (${httpStatus}) ${url}`);
  }
  try {
    return JSON.parse(raw) as ApiResponse<T>;
  } catch {
    // 常见：代理/网关把两段内容拼在一起，或返回了 HTML/纯文本
    const start = raw.indexOf('{');
    if (start >= 0) {
      try {
        return JSON.parse(raw.slice(start)) as ApiResponse<T>;
      } catch {
        // fallthrough
      }
    }
    const preview = raw.length > 280 ? `${raw.slice(0, 280)}…` : raw;
    throw new Error(
      `接口返回非合法 JSON (${httpStatus}) ${url}。前 280 字符：${preview}`,
    );
  }
}

async function request<T>(path: string, body?: unknown, options?: { noAuth?: boolean; useFZM?: string }): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };
  if (options?.useFZM) {
    headers['FZM-SIGNATURE'] = options.useFZM;
  } else if (!options?.noAuth && sessionToken) {
    headers['Authorization'] = `Bearer ${sessionToken}`;
  }

  const url = `${BASE_URL}${path}`;
  console.log(`${TAG} → POST ${url}`, body ? JSON.stringify(body).substring(0, 200) : '(no body)');

  const res = await fetch(url, {
    method: 'POST',
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });

  const text = await res.text();
  const json = parseGatewayJson<T>(text, url, res.status);
  console.log(`${TAG} ← ${url} status=${res.status} result=${json.result}`, JSON.stringify(json.data ?? json.message ?? '').substring(0, 300));

  if (json.result !== 0) {
    console.error(`${TAG} ❌ API 错误: result=${json.result} message=${json.message}`);
    throw new Error(json.message || `API error: ${json.result}`);
  }
  return json.data as T;
}

// -------- 客服认证 API --------

export interface CSAgentPublic {
  agentId: string;
  name: string;
  avatar: string;
  status: number;
}

export interface LoginResp {
  sessionToken: string;
  agent: CSAgentPublic;
  /** ReceiveMsg 后延迟 Ack（ms），与 cs.toml 一致；cs-web 必须以此为唯一来源 */
  cometReceiveAckDelayMillis?: number;
}

export async function login(username: string, password: string): Promise<LoginResp> {
  return request('/cs/login', { username, password }, { noAuth: true });
}

export async function refreshFZMToken(): Promise<{ fzmSignature: string }> {
  return request('/cs/token/refresh');
}

export async function getAgentInfo(): Promise<{ agent: CSAgentPublic; cometReceiveAckDelayMillis?: number }> {
  return request('/cs/agent/info');
}

export async function updateAgentStatus(status: number): Promise<void> {
  return request('/cs/agent/status', { status });
}

/** 探测区块链节点（需已登录 session） */
export async function chainPing(chainRpcUrl: string): Promise<{ height: number }> {
  return request('/cs/chain/ping', { chainRpcUrl });
}

/**
 * 将当前 Comet 根地址写入链上「服务器分组」（需 CS 服务配置代扣私钥 NoBalancePrivateKey）。
 * chainRpcUrl 可省略，由服务端 cs.toml [Chain] RPC 兜底。
 */
export async function bindCometOnChain(cometUrl: string, chainRpcUrl?: string): Promise<{ txHash: string }> {
  const body: Record<string, string> = { cometUrl };
  if (chainRpcUrl?.trim()) body.chainRpcUrl = chainRpcUrl.trim();
  return request('/cs/chain/bind-comet', body);
}

// -------- 消息加密/解密 API --------

export async function encryptMsg(targetAddress: string, data: string): Promise<{ data: string }> {
  return request('/cs/encrypt', { targetAddress, data });
}

export async function decryptMsg(senderAddress: string, data: string): Promise<{ data: string }> {
  return request('/cs/decrypt', { senderAddress, data });
}

export async function getUserPubKey(targetAddress: string): Promise<{ publicKey: string }> {
  return request('/cs/user-pubkey', { targetAddress });
}

export async function registerPubKeyOnChain(): Promise<{ txHash: string }> {
  return request('/cs/register-pubkey', {});
}

export async function verifyChainPubKey(): Promise<{
  agentID: string;
  dbPubKey: string;
  chainRPC: string;
  chainQuery: string;
  chainPubKey?: string;
  chainError?: string;
  match: boolean;
}> {
  return request('/cs/verify-chain-pubkey', {});
}

// -------- 历史消息 API（需要 FZM-SIGNATURE） --------

export async function getPrivateRecords(fzmToken: string, target: string, mid?: string, limit = 20) {
  return request('/store/app/pri-chat-record', { target, mid, limit }, { useFZM: fzmToken });
}
