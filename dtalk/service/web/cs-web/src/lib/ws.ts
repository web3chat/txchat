/**
 * WebSocket 连接管理
 * 封装与 im/comet 的 WebSocket 连接，处理认证、心跳、消息收发、自动重连
 */

import { Proto, Op, encodeProto, decodeProto } from './protocol';

const TAG = '[CS-WS]';

export type ConnectionState = 'disconnected' | 'connecting' | 'authenticating' | 'connected' | 'reconnecting' | 'error';

export interface AuthMsg {
  appId: string;
  token: string;
  ext: Uint8Array;
}

export type MessageHandler = (proto: Proto) => void;
export type StateChangeHandler = (state: ConnectionState, detail?: string) => void;

/**
 * protobuf varint 编码
 */
function pbVarint(value: number): number[] {
  const result: number[] = [];
  while (value > 0x7f) {
    result.push((value & 0x7f) | 0x80);
    value >>>= 7;
  }
  result.push(value & 0x7f);
  return result;
}

/**
 * 编码 imparse.auth.Login protobuf 消息，用作 AuthMsg.ext。
 *
 * proto 定义:
 *   message Login {
 *     Device   device      = 1;  // enum: Android=0..Web=5
 *     string   username    = 2;
 *     string   deviceToken = 3;
 *     ConnType connType    = 4;  // enum: Connect=0, Reconnect=1
 *     string   uuid        = 5;
 *     string   deviceName  = 6;
 *   }
 */
export function encodeLoginExt(
  uuid: string,
  deviceName: string,
  device = 5, /* Web */
  connType = 0, /* Connect */
): Uint8Array {
  const parts: number[] = [];
  const enc = new TextEncoder();

  // field 1: Device (varint), tag = (1<<3)|0 = 0x08
  if (device !== 0) {
    parts.push(0x08, ...pbVarint(device));
  }
  // field 4: ConnType (varint), tag = (4<<3)|0 = 0x20
  if (connType !== 0) {
    parts.push(0x20, ...pbVarint(connType));
  }
  // field 5: uuid (string), tag = (5<<3)|2 = 0x2a
  if (uuid) {
    const b = enc.encode(uuid);
    parts.push(0x2a, ...pbVarint(b.length), ...b);
  }
  // field 6: deviceName (string), tag = (6<<3)|2 = 0x32
  if (deviceName) {
    const b = enc.encode(deviceName);
    parts.push(0x32, ...pbVarint(b.length), ...b);
  }
  return new Uint8Array(parts);
}

const HEARTBEAT_INTERVAL = 30_000;
const HEARTBEAT_TIMEOUT = 10_000;
const RECONNECT_BASE_DELAY = 1_000;
const RECONNECT_MAX_DELAY = 30_000;
const CONNECT_TIMEOUT = 10_000;

export class IMConnection {
  private ws: WebSocket | null = null;
  private seq = 0;
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  private heartbeatTimeoutTimer: ReturnType<typeof setTimeout> | null = null;
  private heartbeatCount = 0;
  private onMessage: MessageHandler | null = null;
  private onDisconnect: (() => void) | null = null;
  private onStateChange: StateChangeHandler | null = null;
  private cometUrl: string;
  private _state: ConnectionState = 'disconnected';

  private authMsg: AuthMsg | null = null;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private reconnectAttempts = 0;
  private autoReconnect = true;
  private manualDisconnect = false;
  private visibilityHandler: (() => void) | null = null;

  /** 用于刷新 token 的回调，重连前如果设置了会先调用它来获取新 token */
  private refreshToken: (() => Promise<string>) | null = null;

  constructor(cometUrl: string) {
    this.cometUrl = cometUrl;
    console.log(`${TAG} 创建 IMConnection, cometUrl=${cometUrl}`);
    this.setupVisibilityHandler();
  }

  get state(): ConnectionState { return this._state; }

  private setState(s: ConnectionState, detail?: string) {
    this._state = s;
    console.log(`${TAG} 状态变更 → ${s}${detail ? ' | ' + detail : ''}`);
    this.onStateChange?.(s, detail);
  }

  setMessageHandler(handler: MessageHandler) { this.onMessage = handler; }
  setDisconnectHandler(handler: () => void) { this.onDisconnect = handler; }
  setStateChangeHandler(handler: StateChangeHandler) { this.onStateChange = handler; }
  setRefreshTokenHandler(handler: () => Promise<string>) { this.refreshToken = handler; }

  private setupVisibilityHandler() {
    this.visibilityHandler = () => {
      if (document.visibilityState === 'visible') {
        console.log(`${TAG} 页面从后台切回前台`);
        if (this._state === 'connected' && this.ws?.readyState === WebSocket.OPEN) {
          this.sendHeartbeatOnce();
        } else if (
          this._state !== 'connected' &&
          this._state !== 'connecting' &&
          this._state !== 'authenticating' &&
          this._state !== 'reconnecting' &&
          !this.manualDisconnect &&
          this.authMsg
        ) {
          console.log(`${TAG} 页面回前台发现已断开，立即重连`);
          this.scheduleReconnect(0);
        }
      }
    };
    document.addEventListener('visibilitychange', this.visibilityHandler);
  }

  async connect(authMsg: AuthMsg): Promise<Proto> {
    this.authMsg = authMsg;
    this.manualDisconnect = false;
    this.reconnectAttempts = 0;
    return this.doConnect();
  }

  private doConnect(): Promise<Proto> {
    return new Promise((resolve, reject) => {
      const isReconnect = this.reconnectAttempts > 0;
      const url = this.cometUrl.replace(/^http/, 'ws') + '/sub';

      console.log(`${TAG} ===== ${isReconnect ? `重连 (#${this.reconnectAttempts})` : '首次连接'} =====`);
      console.log(`${TAG} WebSocket URL: ${url}`);

      this.cleanup();
      this.setState(isReconnect ? 'reconnecting' : 'connecting', `${url} ${isReconnect ? `(第${this.reconnectAttempts}次)` : ''}`);

      this.ws = new WebSocket(url);
      this.ws.binaryType = 'arraybuffer';

      const connectTimeout = setTimeout(() => {
        console.error(`${TAG} WebSocket 连接超时 (${CONNECT_TIMEOUT / 1000}s)`);
        this.setState('error', '连接超时');
        doReject(new Error('websocket connect timeout'));
        this.ws?.close();
      }, CONNECT_TIMEOUT);

      this.ws.onopen = () => {
        console.log(`${TAG} WebSocket onopen - 连接已建立, 开始发送 Auth`);
        this.setState('authenticating', '发送认证消息...');
        this.sendAuth(this.authMsg!);
      };

      this.ws.onmessage = (event: MessageEvent) => {
        const data = event.data as ArrayBuffer;
        const proto = decodeProto(data);

        if (proto.op === Op.AuthReply) {
          clearTimeout(connectTimeout);
          if (proto.body && proto.body.length > 0) {
            const errText = new TextDecoder().decode(proto.body);
            console.error(`${TAG} Auth 失败: ${errText}`);
            this.setState('error', `认证失败: ${errText}`);
            doReject(new Error(`auth failed: ${errText}`));
            return;
          }
          console.log(`${TAG} Auth 成功! 开始心跳`);
          this.reconnectAttempts = 0;
          this.setState('connected', 'Comet 认证通过');
          this.startHeartbeat();
          doResolve(proto);
          return;
        }

        if (proto.op === Op.HeartbeatReply) {
          this.clearHeartbeatTimeout();
          this.heartbeatCount++;
          if (this.heartbeatCount % 5 === 0) {
            console.log(`${TAG} 心跳正常, 累计 ${this.heartbeatCount} 次`);
          }
          return;
        }

        this.onMessage?.(proto);
      };

      let settled = false;
      const doReject = (err: Error) => {
        if (!settled) { settled = true; reject(err); }
      };
      const doResolve = (authReply: Proto) => {
        if (!settled) { settled = true; resolve(authReply); }
      };

      this.ws.onerror = (ev) => {
        clearTimeout(connectTimeout);
        console.error(`${TAG} WebSocket onerror:`, ev);
        this.setState('error', 'WebSocket 连接错误');
        doReject(new Error('websocket error'));
      };

      this.ws.onclose = (ev) => {
        clearTimeout(connectTimeout);
        console.warn(`${TAG} WebSocket onclose: code=${ev.code} reason="${ev.reason}" wasClean=${ev.wasClean}`);
        this.stopHeartbeat();

        if (this.manualDisconnect) {
          this.setState('disconnected', '主动断开');
          doReject(new Error('manually disconnected'));
          return;
        }

        const wasConnected = settled;
        doReject(new Error(`websocket closed: code=${ev.code}`));

        if (wasConnected && this.autoReconnect && this.authMsg) {
          this.setState('reconnecting', `code=${ev.code}`);
          this.scheduleReconnect();
        } else if (!wasConnected) {
          this.setState('disconnected', `code=${ev.code} reason=${ev.reason}`);
          this.onDisconnect?.();
        }
      };
    });
  }

  private async scheduleReconnect(immediateDelay?: number) {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
    }
    if (this.manualDisconnect || !this.authMsg) return;

    this.reconnectAttempts++;
    const delay = immediateDelay ?? Math.min(
      RECONNECT_BASE_DELAY * Math.pow(2, this.reconnectAttempts - 1),
      RECONNECT_MAX_DELAY,
    );
    console.log(`${TAG} 将在 ${delay}ms 后尝试第 ${this.reconnectAttempts} 次重连`);
    this.setState('reconnecting', `${delay / 1000}s 后重连 (第${this.reconnectAttempts}次)`);

    this.reconnectTimer = setTimeout(async () => {
      if (this.manualDisconnect) return;

      if (this.refreshToken) {
        try {
          console.log(`${TAG} 重连前刷新 token...`);
          const newToken = await this.refreshToken();
          this.authMsg = { ...this.authMsg!, token: newToken };
          console.log(`${TAG} token 已刷新`);
        } catch (e) {
          console.error(`${TAG} 刷新 token 失败:`, e);
        }
      }

      try {
        await this.doConnect();
        console.log(`${TAG} 重连成功!`);
      } catch (e) {
        console.error(`${TAG} 重连失败:`, e);
        if (!this.manualDisconnect && this.autoReconnect) {
          this.scheduleReconnect();
        }
      }
    }, delay);
  }

  private sendAuth(authMsg: AuthMsg) {
    const body = this.encodeAuthMsg(authMsg);
    this.send({ ver: 1, op: Op.Auth, seq: 0, ack: 0, body });
  }

  private encodeAuthMsg(msg: AuthMsg): Uint8Array {
    const parts: number[] = [];
    const enc = new TextEncoder();
    const appIdBytes = enc.encode(msg.appId);
    parts.push(0x0a, ...pbVarint(appIdBytes.length), ...appIdBytes);
    const tokenBytes = enc.encode(msg.token);
    parts.push(0x12, ...pbVarint(tokenBytes.length), ...tokenBytes);
    if (msg.ext.length > 0) {
      parts.push(0x1a, ...pbVarint(msg.ext.length), ...msg.ext);
    }
    return new Uint8Array(parts);
  }

  send(proto: Proto) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(encodeProto(proto));
    } else {
      console.warn(`${TAG} send 失败: ws 未就绪, readyState=${this.ws?.readyState}`);
    }
  }

  sendMsg(body: Uint8Array): number {
    this.seq++;
    this.send({ ver: 1, op: Op.SendMsg, seq: this.seq, ack: 0, body });
    return this.seq;
  }

  sendAck(seq: number) {
    this.send({ ver: 1, op: Op.ReceiveMsgReply, seq, ack: seq, body: null });
  }

  private sendHeartbeatOnce() {
    this.seq++;
    this.send({ ver: 1, op: Op.Heartbeat, seq: this.seq, ack: 0, body: null });
    this.startHeartbeatTimeout();
  }

  private startHeartbeat() {
    console.log(`${TAG} 启动心跳定时器 (${HEARTBEAT_INTERVAL / 1000}s 间隔)`);
    this.heartbeatCount = 0;
    this.stopHeartbeat();
    this.heartbeatTimer = setInterval(() => {
      this.sendHeartbeatOnce();
    }, HEARTBEAT_INTERVAL);
  }

  private startHeartbeatTimeout() {
    this.clearHeartbeatTimeout();
    this.heartbeatTimeoutTimer = setTimeout(() => {
      console.warn(`${TAG} 心跳超时 (${HEARTBEAT_TIMEOUT / 1000}s 未收到回复), 断开重连`);
      this.ws?.close(4000, 'heartbeat timeout');
    }, HEARTBEAT_TIMEOUT);
  }

  private clearHeartbeatTimeout() {
    if (this.heartbeatTimeoutTimer) {
      clearTimeout(this.heartbeatTimeoutTimer);
      this.heartbeatTimeoutTimer = null;
    }
  }

  private stopHeartbeat() {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
    this.clearHeartbeatTimeout();
  }

  private cleanup() {
    this.stopHeartbeat();
    if (this.ws) {
      this.ws.onopen = null;
      this.ws.onmessage = null;
      this.ws.onerror = null;
      this.ws.onclose = null;
      if (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING) {
        this.ws.close();
      }
      this.ws = null;
    }
  }

  disconnect() {
    console.log(`${TAG} 主动断开连接`);
    this.manualDisconnect = true;
    this.autoReconnect = false;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.cleanup();
    this.setState('disconnected', '主动断开');
    if (this.visibilityHandler) {
      document.removeEventListener('visibilitychange', this.visibilityHandler);
      this.visibilityHandler = null;
    }
  }
}
