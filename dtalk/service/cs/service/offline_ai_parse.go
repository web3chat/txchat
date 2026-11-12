package service

import (
	"encoding/base64"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/inconshreveable/log15"
	"google.golang.org/protobuf/proto"

	"github.com/web3chat/txchat/dtalk/service/cs/model"
	xproto "github.com/web3chat/txchat/imparse/proto"
)

// offlineAISignalPurpose 对应 imparse/proto/signal.proto 中 SignalType，便于离线 AI 日志可读。
func offlineAISignalPurpose(t xproto.SignalType) string {
	switch t {
	case xproto.SignalType_Received:
		return "消息送达/已读类回执"
	case xproto.SignalType_Revoke:
		return "撤回消息"
	case xproto.SignalType_SignInGroup:
		return "加入群组"
	case xproto.SignalType_SignOutGroup:
		return "退出群组"
	case xproto.SignalType_DeleteGroup:
		return "删除群组"
	case xproto.SignalType_FocusMessage:
		return "聚焦消息"
	case xproto.SignalType_EndpointLogin:
		return "端点登录通知"
	case xproto.SignalType_UpdateGroupJoinType:
		return "更新入群方式"
	case xproto.SignalType_UpdateGroupFriendType:
		return "更新群内加好友策略"
	case xproto.SignalType_UpdateGroupMuteType:
		return "更新全员禁言"
	case xproto.SignalType_UpdateGroupMemberType:
		return "更新成员角色"
	case xproto.SignalType_UpdateGroupMemberMuteTime:
		return "更新成员禁言时长"
	case xproto.SignalType_UpdateGroupName:
		return "更新群名称"
	case xproto.SignalType_UpdateGroupAvatar:
		return "更新群头像"
	case xproto.SignalType_StartCall:
		return "发起通话"
	case xproto.SignalType_AcceptCall:
		return "接听通话"
	case xproto.SignalType_StopCall:
		return "结束通话"
	default:
		return "未枚举的信令子类型"
	}
}

func decodeInboundUserText(agentID string, svc *Service, log log15.Logger, body []byte) (fromAddr string, plainText string, skip bool) {
	var biz xproto.Proto
	if err := proto.Unmarshal(body, &biz); err != nil {
		log.Warn("离线 AI：Biz Proto 解析失败", "err", err)
		return "", "", true
	}
	if biz.GetEventType() != xproto.Proto_common {
		if biz.GetEventType() == xproto.Proto_Signal {
			var sig xproto.Signal
			if err := proto.Unmarshal(biz.GetBody(), &sig); err != nil {
				log.Warn("离线 AI：忽略非 Proto_common 推送",
					"eventType", biz.GetEventType(),
					"signalUnmarshalErr", err,
					"signalBodyLen", len(biz.GetBody()),
				)
			} else {
				st := sig.GetType()
				log.Warn("离线 AI：忽略非 Proto_common 推送",
					"eventType", biz.GetEventType(),
					"signalType", st,
					"signalTypeName", st.String(),
					"signalPurpose", offlineAISignalPurpose(st),
					"signalSid", sig.GetSid(),
					"signalReliable", sig.GetReliable(),
					"signalBodyLen", len(sig.GetBody()),
				)
			}
		} else {
			log.Warn("离线 AI：忽略非 Proto_common 推送", "eventType", biz.GetEventType())
		}
		return "", "", true
	}
	var common xproto.Common
	if err := proto.Unmarshal(biz.GetBody(), &common); err != nil {
		log.Warn("离线 AI：Common 解析失败", "err", err)
		return "", "", true
	}
	from := strings.TrimSpace(common.GetFrom())
	if from == "" || from == agentID {
		return "", "", true
	}
	if common.GetMsgType() != xproto.MsgType_Text || len(common.GetMsg()) == 0 {
		log.Warn("离线 AI：收到消息但非文本或正文为空，跳过自动回复",
			"from", from,
			"msgType", common.GetMsgType(),
			"msgLen", len(common.GetMsg()),
		)
		return from, "", false
	}

	msgBytes := common.GetMsg()
	var tm xproto.TextMsg
	if err := proto.Unmarshal(msgBytes, &tm); err == nil && strings.TrimSpace(tm.GetContent()) != "" && isMostlyPrintable(tm.GetContent()) {
		return from, strings.TrimSpace(tm.GetContent()), false
	}

	b64 := base64.StdEncoding.EncodeToString(msgBytes)
	dec, err := svc.DecryptMsg(agentID, &model.DecryptMsgReq{
		SenderAddress: from,
		Data:          b64,
	})
	if err != nil {
		log.Warn("离线 AI：DecryptMsg 失败", "from", from, "err", err)
		return from, "", false
	}
	decBytes, err := base64.StdEncoding.DecodeString(dec.Data)
	if err != nil {
		log.Warn("离线 AI：解密结果 base64 解码失败", "from", from, "err", err)
		return from, "", false
	}
	if err := proto.Unmarshal(decBytes, &tm); err != nil {
		log.Warn("离线 AI：解密后 TextMsg 解析失败", "from", from, "err", err)
		return from, "", false
	}
	return from, strings.TrimSpace(tm.GetContent()), false
}

func isMostlyPrintable(s string) bool {
	n := 0
	ok := 0
	for _, r := range s {
		n++
		if n > 48 {
			break
		}
		if r == '\n' || r == '\r' || r == '\t' || unicode.IsPrint(r) {
			ok++
		}
	}
	if n == 0 {
		return false
	}
	return ok*10 >= n*8
}

func marshalSendTextPayload(agentID, targetAddr, content string) ([]byte, error) {
	textBytes, err := proto.Marshal(&xproto.TextMsg{Content: content})
	if err != nil {
		return nil, err
	}
	commonBytes, err := proto.Marshal(&xproto.Common{
		ChannelType: xproto.Channel_ToUser,
		Seq:         uuid.New().String(),
		From:        agentID,
		Target:      targetAddr,
		MsgType:     xproto.MsgType_Text,
		Msg:         textBytes,
		Datetime:    uint64(time.Now().UnixMilli()),
	})
	if err != nil {
		return nil, err
	}
	biz := &xproto.Proto{
		EventType: xproto.Proto_common,
		Body:      commonBytes,
	}
	return proto.Marshal(biz)
}
