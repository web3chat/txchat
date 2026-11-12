/**
 * xproto 业务协议编解码
 *
 * 消息结构（接收时）：
 *   xproto.Proto { eventType, body }
 *     → xproto.Common { channelType, mid, seq, from, target, msgType, msg, datetime }
 *       → TextMsg { content }  (当 msgType === 1)
 *
 * 消息结构（发送时）：
 *   xproto.Proto { eventType=0(common), body = Common bytes }
 *     → xproto.Common { channelType=0(ToUser), from, target, msgType=1(Text), msg = TextMsg bytes }
 *       → TextMsg { content }
 */

import { randomUUID } from './randomUUID';

const textDecoder = new TextDecoder();
const textEncoder = new TextEncoder();

// ─── Protobuf wire format helpers ───

function pbReadVarint(buf: Uint8Array, offset: number): [number, number] {
  let result = 0;
  let shift = 0;
  let pos = offset;
  while (pos < buf.length) {
    const b = buf[pos++];
    result |= (b & 0x7f) << shift;
    if ((b & 0x80) === 0) break;
    shift += 7;
    if (shift > 35) {
      // large varint (int64/uint64) — consume remaining bytes, return low 53 bits
      let hi = 0;
      while (pos < buf.length) {
        const b2 = buf[pos++];
        hi |= (b2 & 0x7f) << (shift - 35);
        if ((b2 & 0x80) === 0) break;
        shift += 7;
      }
      return [result + hi * 0x100000000, pos];
    }
  }
  return [result >>> 0, pos];
}

interface PbField {
  fieldNumber: number;
  wireType: number;
  varintValue: number;
  bytesValue: Uint8Array;
}

function pbReadFields(buf: Uint8Array): PbField[] {
  const fields: PbField[] = [];
  let pos = 0;
  while (pos < buf.length) {
    const [tag, p1] = pbReadVarint(buf, pos);
    const fieldNumber = tag >>> 3;
    const wireType = tag & 0x07;
    if (wireType === 0) {
      const [value, p2] = pbReadVarint(buf, p1);
      fields.push({ fieldNumber, wireType, varintValue: value, bytesValue: new Uint8Array(0) });
      pos = p2;
    } else if (wireType === 2) {
      const [len, p2] = pbReadVarint(buf, p1);
      fields.push({ fieldNumber, wireType, varintValue: 0, bytesValue: buf.subarray(p2, p2 + len) });
      pos = p2 + len;
    } else if (wireType === 1) {
      // 64-bit fixed — skip 8 bytes
      fields.push({ fieldNumber, wireType, varintValue: 0, bytesValue: buf.subarray(p1, p1 + 8) });
      pos = p1 + 8;
    } else if (wireType === 5) {
      // 32-bit fixed — skip 4 bytes
      fields.push({ fieldNumber, wireType, varintValue: 0, bytesValue: buf.subarray(p1, p1 + 4) });
      pos = p1 + 4;
    } else {
      break;
    }
  }
  return fields;
}

function pbWriteVarint(value: number): number[] {
  const out: number[] = [];
  let v = value >>> 0;
  while (v > 0x7f) {
    out.push((v & 0x7f) | 0x80);
    v >>>= 7;
  }
  out.push(v & 0x7f);
  return out;
}

function pbWriteTag(fieldNumber: number, wireType: number): number[] {
  return pbWriteVarint((fieldNumber << 3) | wireType);
}

function pbWriteVarintField(fieldNumber: number, value: number): number[] {
  if (value === 0) return [];
  return [...pbWriteTag(fieldNumber, 0), ...pbWriteVarint(value)];
}

function pbWriteBytesField(fieldNumber: number, data: Uint8Array): number[] {
  if (data.length === 0) return [];
  return [...pbWriteTag(fieldNumber, 2), ...pbWriteVarint(data.length), ...data];
}

function pbWriteStringField(fieldNumber: number, value: string): number[] {
  if (!value) return [];
  const encoded = textEncoder.encode(value);
  return pbWriteBytesField(fieldNumber, encoded);
}

function toUint8Array(bytes: number[]): Uint8Array {
  return new Uint8Array(bytes);
}

// ─── Message types ───

export enum EventType {
  Common = 0,
  CommonAck = 1,
  Signal = 2,
  SYSNotice = 3,
}

export enum Channel {
  ToUser = 0,
  ToGroup = 1,
}

export enum MsgType {
  System = 0,
  Text = 1,
  Audio = 2,
  Image = 3,
  Video = 4,
  File = 5,
  Card = 6,
  Notice = 7,
  Forward = 8,
  RTCCall = 9,
  Transfer = 10,
  Collect = 11,
  RedPacket = 12,
  ContactCard = 13,
}

export interface BizProto {
  eventType: EventType;
  body: Uint8Array | null;
}

export interface CommonMsg {
  channelType: Channel;
  mid: number;
  seq: string;
  from: string;
  target: string;
  msgType: MsgType;
  msg: Uint8Array | null;
  datetime: number;
}

export interface TextMsgContent {
  content: string;
}

export interface CommonAck {
  mid: number;
  datetime: number;
}

// ─── Decoders ───

export function decodeBizProto(buf: Uint8Array): BizProto {
  const fields = pbReadFields(buf);
  const result: BizProto = { eventType: EventType.Common, body: null };
  for (const f of fields) {
    if (f.fieldNumber === 1 && f.wireType === 0) result.eventType = f.varintValue as EventType;
    if (f.fieldNumber === 2 && f.wireType === 2) result.body = f.bytesValue;
  }
  return result;
}

export function decodeCommon(buf: Uint8Array): CommonMsg {
  const fields = pbReadFields(buf);
  const result: CommonMsg = {
    channelType: Channel.ToUser,
    mid: 0,
    seq: '',
    from: '',
    target: '',
    msgType: MsgType.System,
    msg: null,
    datetime: 0,
  };
  for (const f of fields) {
    switch (f.fieldNumber) {
      case 1: if (f.wireType === 0) result.channelType = f.varintValue as Channel; break;
      case 2: if (f.wireType === 0) result.mid = f.varintValue; break;
      case 3: if (f.wireType === 2) result.seq = textDecoder.decode(f.bytesValue); break;
      case 4: if (f.wireType === 2) result.from = textDecoder.decode(f.bytesValue); break;
      case 5: if (f.wireType === 2) result.target = textDecoder.decode(f.bytesValue); break;
      case 6: if (f.wireType === 0) result.msgType = f.varintValue as MsgType; break;
      case 7: if (f.wireType === 2) result.msg = f.bytesValue; break;
      case 8: if (f.wireType === 0) result.datetime = f.varintValue; break;
    }
  }
  return result;
}

export function decodeTextMsg(buf: Uint8Array): TextMsgContent {
  const fields = pbReadFields(buf);
  let content = '';
  for (const f of fields) {
    if (f.fieldNumber === 1 && f.wireType === 2) {
      content = textDecoder.decode(f.bytesValue);
    }
  }
  return { content };
}

export function decodeCommonAck(buf: Uint8Array): CommonAck {
  const fields = pbReadFields(buf);
  const result: CommonAck = { mid: 0, datetime: 0 };
  for (const f of fields) {
    if (f.fieldNumber === 2 && f.wireType === 0) result.mid = f.varintValue;
    if (f.fieldNumber === 8 && f.wireType === 0) result.datetime = f.varintValue;
  }
  return result;
}

/**
 * 解析接收到的消息 body（Op.ReceiveMsg 的 body）
 * 返回解码后的 CommonMsg 及提取的文本内容
 */
export function parseReceivedMessage(body: Uint8Array): {
  common: CommonMsg;
  textContent: string | null;
} | null {
  try {
    const biz = decodeBizProto(body);
    if (biz.eventType !== EventType.Common || !biz.body) return null;

    const common = decodeCommon(biz.body);

    let textContent: string | null = null;
    if (common.msgType === MsgType.Text && common.msg) {
      const textMsg = decodeTextMsg(common.msg);
      textContent = textMsg.content;
    } else if (common.msg) {
      textContent = `[${MsgType[common.msgType] ?? '未知类型'}]`;
    }

    return { common, textContent };
  } catch {
    return null;
  }
}

// ─── Encoders ───

export function encodeTextMsg(content: string): Uint8Array {
  return toUint8Array(pbWriteStringField(1, content));
}

export function encodeCommon(msg: {
  channelType?: Channel;
  seq?: string;
  from?: string;
  target: string;
  msgType?: MsgType;
  msgBody: Uint8Array;
}): Uint8Array {
  const bytes: number[] = [
    ...pbWriteVarintField(1, msg.channelType ?? Channel.ToUser),
    ...pbWriteStringField(3, msg.seq ?? ''),
    ...pbWriteStringField(4, msg.from ?? ''),
    ...pbWriteStringField(5, msg.target),
    ...pbWriteVarintField(6, msg.msgType ?? MsgType.Text),
    ...pbWriteBytesField(7, msg.msgBody),
  ];
  return toUint8Array(bytes);
}

export function encodeBizProto(eventType: EventType, body: Uint8Array): Uint8Array {
  const bytes: number[] = [
    ...pbWriteVarintField(1, eventType),
    ...pbWriteBytesField(2, body),
  ];
  return toUint8Array(bytes);
}

/**
 * 构建发送文本消息的 body（用于 Op.SendMsg 的 body）
 */
export function buildSendTextMessage(opts: {
  from: string;
  target: string;
  content: string;
  seq?: string;
}): Uint8Array {
  const textBody = encodeTextMsg(opts.content);
  const commonBody = encodeCommon({
    channelType: Channel.ToUser,
    seq: opts.seq ?? randomUUID(),
    from: opts.from,
    target: opts.target,
    msgType: MsgType.Text,
    msgBody: textBody,
  });
  return encodeBizProto(EventType.Common, commonBody);
}

/**
 * 构建发送文本消息的 body，使用预加密的 msgBody
 */
export function buildSendTextMessageWithBody(opts: {
  from: string;
  target: string;
  msgBody: Uint8Array;
  seq?: string;
}): Uint8Array {
  const commonBody = encodeCommon({
    channelType: Channel.ToUser,
    seq: opts.seq ?? randomUUID(),
    from: opts.from,
    target: opts.target,
    msgType: MsgType.Text,
    msgBody: opts.msgBody,
  });
  return encodeBizProto(EventType.Common, commonBody);
}

// ─── Base64 helpers ───

export function uint8ToBase64(buf: Uint8Array): string {
  let binary = '';
  for (let i = 0; i < buf.length; i++) {
    binary += String.fromCharCode(buf[i]);
  }
  return btoa(binary);
}

export function base64ToUint8(b64: string): Uint8Array {
  const binary = atob(b64);
  const buf = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    buf[i] = binary.charCodeAt(i);
  }
  return buf;
}
