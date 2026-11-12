import { IMConnection } from './lib/ws';
import type { CSAgentPublic } from './lib/api';

/**
 * 全局状态（简单单例模式，避免引入额外状态管理库）
 */

export interface ChatMessage {
  id: string;
  from: string;
  to: string;
  content: string;
  timestamp: number;
  isSelf: boolean;
}

export interface AuthReplyInfo {
  ver: number;
  op: number;
  opName: string;
  seq: number;
  ack: number;
  bodyLength: number;
  bodyHex: string;
  bodyText: string;
  success: boolean;
}

export interface LoginResult {
  csLogin: {
    sessionToken: string;
    agent: CSAgentPublic;
  };
  fzmSignature: {
    signature: string;
    length: number;
  };
  cometConnect: {
    wsUrl: string;
    appId: string;
    deviceUUID: string;
    deviceName: string;
    extLength: number;
    authReply: AuthReplyInfo;
  };
  agentStatusUpdate: {
    success: boolean;
    detail: string;
  };
  loginTime: string;
  /** 服务端下发的 ReceiveMsg→Ack 延迟（ms），与 cs.toml 一致 */
  cometReceiveAckDelayMillis?: number;
}

export interface ConnectParams {
  cometUrl: string;
  appId: string;
  deviceUUID: string;
  deviceName: string;
  /** 区块链 JSON-RPC，与 ChatPro 合约节点一致（如 http://host:8901） */
  chainRpcUrl?: string;
}

export const connectionStore: {
  connection: IMConnection | null;
  loginResult: LoginResult | null;
  connectParams: ConnectParams | null;
} = {
  connection: null,
  loginResult: null,
  connectParams: null,
};
