// Package proto 把 v1.0.0 拆开的子包再导出到 github.com/web3chat/txchat/imparse/proto，
// 让 dtalk / im 继续使用旧 import，无需改业务代码。
package proto

import (
	"github.com/web3chat/txchat/imparse/proto/auth"
	"github.com/web3chat/txchat/imparse/proto/common"
	"github.com/web3chat/txchat/imparse/proto/msg"
	"github.com/web3chat/txchat/imparse/proto/signal"
)

type (
	Channel    = common.Channel
	MsgType    = common.MsgType
	Proto      = common.Proto
	Common     = common.Common
	Source     = common.Source
	SourceUser = common.SourceUser
	CommonAck  = common.CommonAck
	Reference  = common.Reference

	Proto_EventType = common.Proto_EventType

	Device         = auth.Device
	Login          = auth.Login
	Login_ConnType = auth.Login_ConnType

	SignalType                      = signal.SignalType
	JoinType                        = signal.JoinType
	FriendType                      = signal.FriendType
	MuteType                        = signal.MuteType
	MemberType                      = signal.MemberType
	StopCallType                    = signal.StopCallType
	RevokeReason                    = signal.RevokeReason
	Signal                          = signal.Signal
	SignalReceived                  = signal.SignalReceived
	SignalSignInGroup               = signal.SignalSignInGroup
	SignalSignOutGroup              = signal.SignalSignOutGroup
	SignalDeleteGroup               = signal.SignalDeleteGroup
	SignalFocusMessage              = signal.SignalFocusMessage
	SignalEndpointLogin             = signal.SignalEndpointLogin
	SignalUpdateGroupJoinType       = signal.SignalUpdateGroupJoinType
	SignalUpdateGroupFriendType     = signal.SignalUpdateGroupFriendType
	SignalUpdateGroupMuteType       = signal.SignalUpdateGroupMuteType
	SignalUpdateGroupMemberType     = signal.SignalUpdateGroupMemberType
	SignalUpdateGroupMemberMuteTime = signal.SignalUpdateGroupMemberMuteTime
	SignalUpdateGroupName           = signal.SignalUpdateGroupName
	SignalUpdateGroupAvatar         = signal.SignalUpdateGroupAvatar
	SignalStartCall                 = signal.SignalStartCall
	SignalAcceptCall                = signal.SignalAcceptCall
	SignalStopCall                  = signal.SignalStopCall
	SignalRevoke                    = signal.SignalRevoke

	NoticeMsgType                       = msg.NoticeMsgType
	TransferMsg_CoinType                = msg.TransferMsg_CoinType
	RedPacketMsg_RPType                 = msg.RedPacketMsg_RPType
	RedPacketMsg_CoinType               = msg.RedPacketMsg_CoinType
	ContactCardMsg_CardType             = msg.ContactCardMsg_CardType
	EncryptMsg                          = msg.EncryptMsg
	TextMsg                             = msg.TextMsg
	AudioMsg                            = msg.AudioMsg
	ImageMsg                            = msg.ImageMsg
	VideoMsg                            = msg.VideoMsg
	FileMsg                             = msg.FileMsg
	CardMsg                             = msg.CardMsg
	NoticeMsg                           = msg.NoticeMsg
	ForwardItem                         = msg.ForwardItem
	ForwardMsg                          = msg.ForwardMsg
	TransferMsg                         = msg.TransferMsg
	RedPacketMsg                        = msg.RedPacketMsg
	ContactCardMsg                      = msg.ContactCardMsg
	NoticeMsgUpdateGroupName            = msg.NoticeMsgUpdateGroupName
	NoticeMsgSignInGroup                = msg.NoticeMsgSignInGroup
	NoticeMsgSignOutGroup               = msg.NoticeMsgSignOutGroup
	NoticeMsgKickOutGroup               = msg.NoticeMsgKickOutGroup
	NoticeMsgDeleteGroup                = msg.NoticeMsgDeleteGroup
	NoticeMsgUpdateGroupMuted           = msg.NoticeMsgUpdateGroupMuted
	NoticeMsgUpdateGroupMemberMutedTime = msg.NoticeMsgUpdateGroupMemberMutedTime
	NoticeMsgUpdateGroupOwner           = msg.NoticeMsgUpdateGroupOwner
)

const (
	Channel_ToUser  = common.Channel_ToUser
	Channel_ToGroup = common.Channel_ToGroup

	MsgType_System      = common.MsgType_System
	MsgType_Text        = common.MsgType_Text
	MsgType_Audio       = common.MsgType_Audio
	MsgType_Image       = common.MsgType_Image
	MsgType_Video       = common.MsgType_Video
	MsgType_File        = common.MsgType_File
	MsgType_Card        = common.MsgType_Card
	MsgType_Notice      = common.MsgType_Notice
	MsgType_Forward     = common.MsgType_Forward
	MsgType_RTCCall     = common.MsgType_RTCCall
	MsgType_Transfer    = common.MsgType_Transfer
	MsgType_Collect     = common.MsgType_Collect
	MsgType_RedPacket   = common.MsgType_RedPacket
	MsgType_ContactCard = common.MsgType_ContactCard

	Proto_common    = common.Proto_common
	Proto_commonAck = common.Proto_commonAck
	Proto_Signal    = common.Proto_Signal
	Proto_SYSNotice = common.Proto_SYSNotice

	Device_Android = auth.Device_Android
	Device_IOS     = auth.Device_IOS
	Device_Windows = auth.Device_Windows
	Device_Linux   = auth.Device_Linux
	Device_MacOS   = auth.Device_MacOS
	Device_Web     = auth.Device_Web

	Login_Connect   = auth.Login_Connect
	Login_Reconnect = auth.Login_Reconnect

	SignalType_Received                  = signal.SignalType_Received
	SignalType_Revoke                    = signal.SignalType_Revoke
	SignalType_SignInGroup               = signal.SignalType_SignInGroup
	SignalType_SignOutGroup              = signal.SignalType_SignOutGroup
	SignalType_DeleteGroup               = signal.SignalType_DeleteGroup
	SignalType_FocusMessage              = signal.SignalType_FocusMessage
	SignalType_EndpointLogin             = signal.SignalType_EndpointLogin
	SignalType_UpdateGroupJoinType       = signal.SignalType_UpdateGroupJoinType
	SignalType_UpdateGroupFriendType     = signal.SignalType_UpdateGroupFriendType
	SignalType_UpdateGroupMuteType       = signal.SignalType_UpdateGroupMuteType
	SignalType_UpdateGroupMemberType     = signal.SignalType_UpdateGroupMemberType
	SignalType_UpdateGroupMemberMuteTime = signal.SignalType_UpdateGroupMemberMuteTime
	SignalType_UpdateGroupName           = signal.SignalType_UpdateGroupName
	SignalType_UpdateGroupAvatar         = signal.SignalType_UpdateGroupAvatar
	SignalType_StartCall                 = signal.SignalType_StartCall
	SignalType_AcceptCall                = signal.SignalType_AcceptCall
	SignalType_StopCall                  = signal.SignalType_StopCall

	JoinType_JoinAllow = signal.JoinType_JoinAllow
	JoinType_JoinDeny  = signal.JoinType_JoinDeny
	JoinType_JoinApply = signal.JoinType_JoinApply

	FriendType_FriendAllow = signal.FriendType_FriendAllow
	FriendType_FriendDeny  = signal.FriendType_FriendDeny

	MuteType_MuteAllow = signal.MuteType_MuteAllow
	MuteType_MuteDeny  = signal.MuteType_MuteDeny

	MemberType_Normal = signal.MemberType_Normal
	MemberType_Admin  = signal.MemberType_Admin
	MemberType_Owner  = signal.MemberType_Owner

	StopCallType_Busy    = signal.StopCallType_Busy
	StopCallType_Timeout = signal.StopCallType_Timeout
	StopCallType_Reject  = signal.StopCallType_Reject
	StopCallType_Hangup  = signal.StopCallType_Hangup
	StopCallType_Cancel  = signal.StopCallType_Cancel

	RevokeReason_revoked = signal.RevokeReason_revoked
	RevokeReason_burned  = signal.RevokeReason_burned

	NoticeMsgType_UpdateGroupNameNoticeMsg        = msg.NoticeMsgType_UpdateGroupNameNoticeMsg
	NoticeMsgType_SignInGroupNoticeMsg            = msg.NoticeMsgType_SignInGroupNoticeMsg
	NoticeMsgType_SignOutGroupNoticeMsg           = msg.NoticeMsgType_SignOutGroupNoticeMsg
	NoticeMsgType_KickOutGroupNoticeMsg           = msg.NoticeMsgType_KickOutGroupNoticeMsg
	NoticeMsgType_DeleteGroupNoticeMsg            = msg.NoticeMsgType_DeleteGroupNoticeMsg
	NoticeMsgType_UpdateGroupMutedNoticeMsg       = msg.NoticeMsgType_UpdateGroupMutedNoticeMsg
	NoticeMsgType_UpdateGroupMemberMutedNoticeMsg = msg.NoticeMsgType_UpdateGroupMemberMutedNoticeMsg
	NoticeMsgType_UpdateGroupOwnerNoticeMsg       = msg.NoticeMsgType_UpdateGroupOwnerNoticeMsg
	NoticeMsgType_MsgRevoked                      = msg.NoticeMsgType_MsgRevoked

	TransferMsg_Coins = msg.TransferMsg_Coins
	TransferMsg_Token = msg.TransferMsg_Token

	RedPacketMsg_RandomAmount    = msg.RedPacketMsg_RandomAmount
	RedPacketMsg_IdenticalAmount = msg.RedPacketMsg_IdenticalAmount
	RedPacketMsg_Coins           = msg.RedPacketMsg_Coins
	RedPacketMsg_Token           = msg.RedPacketMsg_Token

	ContactCardMsg_Undefined = msg.ContactCardMsg_Undefined
	ContactCardMsg_Personal  = msg.ContactCardMsg_Personal
)

var (
	Channel_name  = common.Channel_name
	Channel_value = common.Channel_value

	MsgType_name  = common.MsgType_name
	MsgType_value = common.MsgType_value

	Proto_EventType_name  = common.Proto_EventType_name
	Proto_EventType_value = common.Proto_EventType_value

	Device_name  = auth.Device_name
	Device_value = auth.Device_value

	Login_ConnType_name  = auth.Login_ConnType_name
	Login_ConnType_value = auth.Login_ConnType_value

	SignalType_name  = signal.SignalType_name
	SignalType_value = signal.SignalType_value

	JoinType_name  = signal.JoinType_name
	JoinType_value = signal.JoinType_value

	FriendType_name  = signal.FriendType_name
	FriendType_value = signal.FriendType_value

	MuteType_name  = signal.MuteType_name
	MuteType_value = signal.MuteType_value

	MemberType_name  = signal.MemberType_name
	MemberType_value = signal.MemberType_value

	StopCallType_name  = signal.StopCallType_name
	StopCallType_value = signal.StopCallType_value

	RevokeReason_name  = signal.RevokeReason_name
	RevokeReason_value = signal.RevokeReason_value

	NoticeMsgType_name  = msg.NoticeMsgType_name
	NoticeMsgType_value = msg.NoticeMsgType_value

	TransferMsg_CoinType_name  = msg.TransferMsg_CoinType_name
	TransferMsg_CoinType_value = msg.TransferMsg_CoinType_value

	RedPacketMsg_RPType_name  = msg.RedPacketMsg_RPType_name
	RedPacketMsg_RPType_value = msg.RedPacketMsg_RPType_value

	RedPacketMsg_CoinType_name  = msg.RedPacketMsg_CoinType_name
	RedPacketMsg_CoinType_value = msg.RedPacketMsg_CoinType_value

	ContactCardMsg_CardType_name  = msg.ContactCardMsg_CardType_name
	ContactCardMsg_CardType_value = msg.ContactCardMsg_CardType_value
)
