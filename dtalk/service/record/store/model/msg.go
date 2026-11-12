package model

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/golang/protobuf/proto"
	zlog "github.com/rs/zerolog/log"
	xproto "github.com/web3chat/txchat/imparse/proto"
)

var msgFactory = map[xproto.MsgType]func() proto.Message{
	xproto.MsgType_Text: func() proto.Message {
		return &xproto.TextMsg{}
	},
	xproto.MsgType_Audio: func() proto.Message {
		return &xproto.AudioMsg{}
	},
	xproto.MsgType_Image: func() proto.Message {
		return &xproto.ImageMsg{}
	},
	xproto.MsgType_Video: func() proto.Message {
		return &xproto.VideoMsg{}
	},
	xproto.MsgType_File: func() proto.Message {
		return &xproto.FileMsg{}
	},
	xproto.MsgType_Card: func() proto.Message {
		return &xproto.CardMsg{}
	},
	xproto.MsgType_Notice: func() proto.Message {
		return &xproto.NoticeMsg{}
	},
	xproto.MsgType_Forward: func() proto.Message {
		return &xproto.ForwardMsg{}
	},
	xproto.MsgType_Transfer: func() proto.Message {
		return &xproto.TransferMsg{}
	},
	xproto.MsgType_RedPacket: func() proto.Message {
		return &xproto.RedPacketMsg{}
	},
	xproto.MsgType_ContactCard: func() proto.Message {
		return &xproto.ContactCardMsg{}
	},
}

var signalFactory = map[xproto.SignalType]func() proto.Message{
	xproto.SignalType_Received: func() proto.Message {
		return &xproto.SignalReceived{}
	},
	xproto.SignalType_Revoke: func() proto.Message {
		return &xproto.SignalRevoke{}
	},
	xproto.SignalType_SignInGroup: func() proto.Message {
		return &xproto.SignalSignInGroup{}
	},
	xproto.SignalType_SignOutGroup: func() proto.Message {
		return &xproto.SignalSignOutGroup{}
	},
	xproto.SignalType_DeleteGroup: func() proto.Message {
		return &xproto.SignalDeleteGroup{}
	},
	xproto.SignalType_FocusMessage: func() proto.Message {
		return &xproto.SignalFocusMessage{}
	},
	xproto.SignalType_UpdateGroupJoinType: func() proto.Message {
		return &xproto.SignalUpdateGroupJoinType{}
	},
	xproto.SignalType_UpdateGroupFriendType: func() proto.Message {
		return &xproto.SignalUpdateGroupFriendType{}
	},
	xproto.SignalType_UpdateGroupMuteType: func() proto.Message {
		return &xproto.SignalUpdateGroupMuteType{}
	},
	xproto.SignalType_UpdateGroupMemberType: func() proto.Message {
		return &xproto.SignalUpdateGroupMemberType{}
	},
	xproto.SignalType_UpdateGroupMemberMuteTime: func() proto.Message {
		return &xproto.SignalUpdateGroupMemberMuteTime{}
	},
	xproto.SignalType_UpdateGroupName: func() proto.Message {
		return &xproto.SignalUpdateGroupName{}
	},
	xproto.SignalType_UpdateGroupAvatar: func() proto.Message {
		return &xproto.SignalUpdateGroupAvatar{}
	},
	xproto.SignalType_StartCall: func() proto.Message {
		return &xproto.SignalStartCall{}
	},
	xproto.SignalType_AcceptCall: func() proto.Message {
		return &xproto.SignalAcceptCall{}
	},
	xproto.SignalType_StopCall: func() proto.Message {
		return &xproto.SignalStopCall{}
	},
}

func parse(m proto.Message, data []byte) []byte {
	err := proto.Unmarshal(data, m)
	if err != nil {
		// 客户端常为端到端密文，按明文 MsgType 反序列化失败属预期；降为 Trace，避免刷屏
		zlog.Trace().Int("len", len(data)).Msg("ParseCommonMsg fallback base64 (likely ciphertext)")
		return []byte(base64.StdEncoding.EncodeToString(data))
	}
	b, err := json.Marshal(m)
	if err != nil {
		zlog.Error().Err(err).Msg("arseMsg json.Marshal err")
		return data
	}
	return b
}

func convert(m proto.Message, data []byte) []byte {
	err := json.Unmarshal(data, m)
	if err != nil {
		zlog.Debug().Err(err).Msg("ConvertMsg json.Unmarshal")
		v, err := base64.StdEncoding.DecodeString(string(data))
		if err != nil {
			zlog.Error().Err(err).Msg("ConvertMsg base64.StdEncoding.DecodeString")
			return data
		}
		return v
	}
	b, err := proto.Marshal(m)
	if err != nil {
		zlog.Error().Err(err).Msg("ConvertMsg proto.Marshal")
		return data
	}
	return b
}

func decodeBase64Flexible(data []byte) ([]byte, bool) {
	s := strings.TrimSpace(string(data))
	if s == "" {
		return nil, false
	}
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, enc := range encodings {
		decoded, err := enc.DecodeString(s)
		if err == nil && len(decoded) > 0 {
			return decoded, true
		}
	}
	return nil, false
}

func jsonConvert(creator func() proto.Message, data []byte) proto.Message {
	if len(data) == 0 {
		return nil
	}

	msg := creator()
	if err := json.Unmarshal(data, msg); err == nil && hasMessagePayload(msg) {
		return msg
	}

	msg = creator()
	if err := proto.Unmarshal(data, msg); err == nil && hasMessagePayload(msg) {
		return msg
	}

	if decoded, ok := decodeBase64Flexible(data); ok {
		msg = creator()
		if err := proto.Unmarshal(decoded, msg); err == nil && hasMessagePayload(msg) {
			return msg
		}
	}

	var jsonStr string
	if err := json.Unmarshal(data, &jsonStr); err == nil && jsonStr != "" {
		if decoded, ok := decodeBase64Flexible([]byte(jsonStr)); ok {
			msg = creator()
			if err := proto.Unmarshal(decoded, msg); err == nil && hasMessagePayload(msg) {
				return msg
			}
		}
	}

	return nil
}

func hasMessagePayload(msg proto.Message) bool {
	if msg == nil {
		return false
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return false
	}
	s := strings.TrimSpace(string(b))
	return s != "" && s != "{}" && s != "null"
}

func protoToInterface(msg proto.Message) interface{} {
	if msg == nil {
		return nil
	}
	b, err := json.Marshal(msg)
	if err != nil {
		zlog.Error().Err(err).Msg("protoToInterface json.Marshal failed")
		return nil
	}
	var out interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		zlog.Error().Err(err).Msg("protoToInterface json.Unmarshal failed")
		return nil
	}
	return out
}

// ParseContent 将 DB 中的 content 解析为 API 可返回的 JSON 对象。
// 兼容 JSON / Protobuf / Base64(Protobuf) / JSON 字符串包 Base64；结构化解析失败时回退原始 JSON 或密文包装。
func ParseContent(msgType uint32, data []byte) interface{} {
	if len(data) == 0 {
		return nil
	}

	creator, ok := msgFactory[xproto.MsgType(msgType)]
	if !ok || creator == nil {
		var raw interface{}
		if err := json.Unmarshal(data, &raw); err == nil {
			return raw
		}
		return opaqueContentPayload(data)
	}

	if msg := jsonConvert(creator, data); msg != nil {
		if out := protoToInterface(msg); out != nil {
			return out
		}
	}

	if out := tryParseEncryptMsg(data); out != nil {
		return out
	}

	var jsonData interface{}
	if err := json.Unmarshal(data, &jsonData); err == nil {
		if str, ok := jsonData.(string); ok && isBase64String(str) {
			if decoded, ok := decodeBase64Flexible([]byte(str)); ok {
				if msg := jsonConvert(creator, decoded); msg != nil {
					if out := protoToInterface(msg); out != nil {
						return out
					}
				}
				if out := tryParseEncryptMsg(decoded); out != nil {
					return out
				}
			}
		}
		return jsonData
	}

	if decoded, ok := decodeBase64Flexible(data); ok {
		if msg := jsonConvert(creator, decoded); msg != nil {
			if out := protoToInterface(msg); out != nil {
				return out
			}
		}
		if out := tryParseEncryptMsg(decoded); out != nil {
			return out
		}
	}

	if out := tryParseEncryptMsg(data); out != nil {
		return out
	}

	// 端到端密文按 MsgType 结构化解析失败属预期，静默回退 opaque，避免 gateway 刷屏
	return opaqueContentPayload(data)
}

func tryParseEncryptMsg(data []byte) interface{} {
	if len(data) == 0 {
		return nil
	}
	msg := &xproto.EncryptMsg{}
	if err := proto.Unmarshal(data, msg); err == nil && msg.Content != "" {
		return map[string]interface{}{"content": msg.Content}
	}
	return nil
}

func opaqueContentPayload(data []byte) interface{} {
	s := strings.TrimSpace(string(data))
	if isBase64String(s) {
		return map[string]interface{}{"content": s}
	}
	return map[string]interface{}{"content": base64.StdEncoding.EncodeToString(data)}
}

func isBase64String(s string) bool {
	if len(s) < 4 {
		return false
	}
	base64Chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/="
	for _, c := range s {
		if !strings.ContainsRune(base64Chars, c) {
			return false
		}
	}
	return true
}

func ParseCommonMsg(m *xproto.Common) []byte {
	creator, ok := msgFactory[m.MsgType]
	if !ok || creator == nil {
		zlog.Debug().Int32("type", int32(m.MsgType)).Interface("creator", creator).Msg("ParseCommon unknown type")
		return m.Msg
	}
	msg := creator()
	return parse(msg, m.Msg)
}

func ParseSource(m *xproto.Common) []byte {
	if m.Source == nil {
		return nil
	}
	b, err := json.Marshal(m.Source)
	if err != nil {
		zlog.Error().Err(err).Msg("ParseCommon json.Marshal err")
		return b
	}
	return b
}

func ParseReference(m *xproto.Common) []byte {
	if m.Reference == nil {
		return nil
	}
	b, err := json.Marshal(m.Reference)
	if err != nil {
		zlog.Error().Err(err).Msg("ParseCommon json.Marshal err")
		return b
	}
	return b
}

func ParseSignal(m *xproto.Signal) []byte {
	creator, ok := signalFactory[m.Type]
	if !ok || creator == nil {
		zlog.Error().Interface("action", m.Type).Interface("creator", creator).Msg("ParseSignal unknown type")
		return m.Body
	}
	msg := creator()
	return parse(msg, m.Body)
}

func JsonUnmarshal(msgType uint32, data []byte) proto.Message {
	creator, ok := msgFactory[xproto.MsgType(msgType)]
	if !ok || creator == nil {
		zlog.Error().Uint32("type", msgType).Interface("creator", creator).Msg("JsonUnmarshal unknown type")
		return nil
	}
	return jsonConvert(creator, data)
}

func ConvertMsg(msgType uint32, data []byte) []byte {
	creator, ok := msgFactory[xproto.MsgType(msgType)]
	if !ok || creator == nil {
		zlog.Error().Uint32("type", msgType).Interface("creator", creator).Msg("ConvertMsg unknown type")
		return data
	}
	msg := creator()
	return convert(msg, data)
}

func ConvertSource(data []byte) *xproto.Source {
	if len(data) == 0 {
		return nil
	}
	var src xproto.Source
	err := json.Unmarshal(data, &src)
	if err != nil {
		zlog.Error().Err(err).Msg("ParseMsg json.Unmarshal err")
		return &src
	}
	return &src
}

func ConvertReference(data []byte) *xproto.Reference {
	if len(data) == 0 {
		return nil
	}
	var src xproto.Reference
	err := json.Unmarshal(data, &src)
	if err != nil {
		zlog.Error().Err(err).Msg("ParseMsg json.Unmarshal err")
		return &src
	}
	return &src
}

func ConvertSignal(actionType uint32, data []byte) []byte {
	creator, ok := signalFactory[xproto.SignalType(actionType)]
	if !ok || creator == nil {
		zlog.Error().Uint32("action", actionType).Interface("creator", creator).Msg("ConvertMsg unknown type")
		return data
	}
	msg := creator()
	return convert(msg, data)
}
