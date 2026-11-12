/**
 * im/comet 二进制协议编解码
 *
 * 帧格式 (大端序):
 * PackSize(4) | HeaderSize(2) | Ver(2) | Op(4) | Seq(4) | Ack(4) | Body(n)
 *
 * 总头部长度 = 4+2+2+4+4+4 = 20 字节
 */

const RAW_HEADER_SIZE = 20;

const PACK_OFFSET = 0;
const HEADER_OFFSET = 4;
const VER_OFFSET = 6;
const OP_OFFSET = 8;
const SEQ_OFFSET = 12;
const ACK_OFFSET = 16;

export enum Op {
  Auth = 1,
  AuthReply = 2,
  Heartbeat = 3,
  HeartbeatReply = 4,
  Disconnect = 5,
  DisconnectReply = 6,
  SendMsg = 7,
  SendMsgReply = 8,
  ReceiveMsg = 9,
  ReceiveMsgReply = 10,
  ProtoReady = 11,
  ProtoFinish = 12,
  Raw = 13,
  SyncMsgReq = 14,
  SyncMsgReply = 15,
  RePush = 16,
}

export interface Proto {
  ver: number;
  op: number;
  seq: number;
  ack: number;
  body: Uint8Array | null;
}

export function encodeProto(p: Proto): ArrayBuffer {
  const bodyLen = p.body ? p.body.byteLength : 0;
  const packLen = RAW_HEADER_SIZE + bodyLen;
  const buf = new ArrayBuffer(packLen);
  const view = new DataView(buf);

  view.setInt32(PACK_OFFSET, packLen);
  view.setInt16(HEADER_OFFSET, RAW_HEADER_SIZE);
  view.setInt16(VER_OFFSET, p.ver);
  view.setInt32(OP_OFFSET, p.op);
  view.setInt32(SEQ_OFFSET, p.seq);
  view.setInt32(ACK_OFFSET, p.ack);

  if (p.body && bodyLen > 0) {
    new Uint8Array(buf, RAW_HEADER_SIZE).set(p.body);
  }
  return buf;
}

export function decodeProto(data: ArrayBuffer): Proto {
  const view = new DataView(data);
  const packLen = view.getInt32(PACK_OFFSET);
  const headerLen = view.getInt16(HEADER_OFFSET);
  const ver = view.getInt16(VER_OFFSET);
  const op = view.getInt32(OP_OFFSET);
  const seq = view.getInt32(SEQ_OFFSET);
  const ack = view.getInt32(ACK_OFFSET);

  let body: Uint8Array | null = null;
  const bodyLen = packLen - headerLen;
  if (bodyLen > 0) {
    body = new Uint8Array(data, headerLen, bodyLen);
  }

  return { ver, op, seq, ack, body };
}
