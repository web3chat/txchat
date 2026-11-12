package record

import (
	"context"
	"time"

	xproto "github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/model"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/svc"
	"github.com/web3chat/txchat/dtalk/pkg/util"
	"github.com/web3chat/txchat/dtalk/service/group/model/biz"
	storeModel "github.com/web3chat/txchat/dtalk/service/record/store/model"
	"github.com/web3chat/txchat/imparse/proto"
)

// recordStore 抽象 store 服务中 RevokePersonal 依赖的方法，便于单元测试注入桩实现。
type recordStore interface {
	GetRecord(ctx context.Context, tp proto.Channel, mid int64) (*storeModel.MsgContent, error)
	DelRecord(ctx context.Context, tp proto.Channel, mid int64) error
}

// signalCaster 抽象 answer 服务中 RevokePersonal 依赖的单播信令方法，便于单元测试注入桩实现。
type signalCaster interface {
	UniCastSignal(ctx context.Context, action proto.SignalType, target string, body []byte) error
}

type Logic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext

	store        recordStore
	caster       signalCaster
	burner       burnStore
	revokeExpire time.Duration
}

func NewLogic(ctx context.Context, svcCtx *svc.ServiceContext) Logic {
	return Logic{
		ctx:          ctx,
		svcCtx:       svcCtx,
		store:        svcCtx.StoreClient,
		caster:       svcCtx.AnswerClient,
		burner:       svcCtx.StoreClient,
		revokeExpire: time.Duration(svcCtx.Config().Revoke.Expire),
	}
}

func personalRevokeAllowed(senderID, receiverID, operator string, createTime uint64, expire time.Duration) bool {
	isParticipant := senderID == operator || receiverID == operator
	if !isParticipant {
		return false
	}
	return time.Since(util.UnixToTime(int64(createTime))) <= expire
}

func (l *Logic) RevokePersonal(Operator string, mid int64) error {
	rd, err := l.store.GetRecord(l.ctx, proto.Channel_ToUser, mid)
	if err != nil {
		log.Error().
			Err(err).
			Str("operator", Operator).
			Int64("mid", mid).
			Msg("revoke personal get record failed")
		return err
	}

	if !personalRevokeAllowed(rd.SenderId, rd.ReceiverId, Operator, rd.CreateTime, l.revokeExpire) {
		log.Warn().
			Str("operator", Operator).
			Int64("mid", mid).
			Str("senderId", rd.SenderId).
			Str("receiverId", rd.ReceiverId).
			Msg("revoke personal permission denied")
		return model.ErrPermission
	}

	action := &proto.SignalRevoke{
		Mid:      mid,
		Operator: Operator,
		Self:     rd.SenderId == Operator,
	}
	body, err := xproto.Marshal(action)
	if err != nil {
		log.Error().
			Err(err).
			Str("operator", Operator).
			Int64("mid", mid).
			Msg("revoke personal marshal signal failed")
		return errors.WithMessagef(err, "proto.Marshal, action=%+v", action)
	}

	if rd.SenderId == Operator {
		// 发送者撤回：保持旧顺序 Receiver → Operator
		if err := l.caster.UniCastSignal(l.ctx, proto.SignalType_Revoke, rd.ReceiverId, body); err != nil {
			log.Error().
				Err(err).
				Str("operator", Operator).
				Int64("mid", mid).
				Str("target", rd.ReceiverId).
				Msg("revoke personal unicast signal to receiver failed")
			return err
		}
	} else {
		// 接收者撤销对方消息：Sender → Receiver
		if err := l.caster.UniCastSignal(l.ctx, proto.SignalType_Revoke, rd.SenderId, body); err != nil {
			log.Error().
				Err(err).
				Str("operator", Operator).
				Int64("mid", mid).
				Str("target", rd.SenderId).
				Msg("revoke personal unicast signal to sender failed")
			return err
		}
	}
	if err := l.store.DelRecord(l.ctx, proto.Channel_ToUser, mid); err != nil {
		log.Error().
			Err(err).
			Str("operator", Operator).
			Int64("mid", mid).
			Msg("revoke personal del record failed")
		return err
	}
	if l.svcCtx != nil && l.svcCtx.FedOut != nil && l.svcCtx.FedOut.Enabled() {
		appId := "dtalk"
		if len(rd.SenderId) >= 2 && rd.SenderId[:2] == "0x" {
			appId = "dtalk-eth"
		}
		if ferr := l.svcCtx.FedOut.ForwardSyncState(l.ctx, appId, rd.SenderId, rd.ReceiverId, "revoke", mid, body); ferr != nil {
			log.Error().Err(ferr).Str("operator", Operator).Int64("mid", mid).
				Str("senderId", rd.SenderId).Str("receiverId", rd.ReceiverId).
				Msg("revoke personal federation ForwardSyncState failed")
		}
	}

	if rd.SenderId == Operator {
		// 与旧逻辑一致：第二次通知 Operator；自聊时目标与第一次相同，会重复下发
		if err := l.caster.UniCastSignal(l.ctx, proto.SignalType_Revoke, Operator, body); err != nil {
			log.Error().
				Err(err).
				Str("operator", Operator).
				Int64("mid", mid).
				Str("target", Operator).
				Msg("revoke personal unicast signal to operator failed")
			return err
		}
		log.Info().
			Str("operator", Operator).
			Int64("mid", mid).
			Str("senderId", rd.SenderId).
			Str("receiverId", rd.ReceiverId).
			Bool("self", action.Self).
			Msg("revoke personal succeeded")
		return nil
	}
	if err := l.caster.UniCastSignal(l.ctx, proto.SignalType_Revoke, rd.ReceiverId, body); err != nil {
		log.Error().
			Err(err).
			Str("operator", Operator).
			Int64("mid", mid).
			Str("target", rd.ReceiverId).
			Msg("revoke personal unicast signal to receiver failed")
		return err
	}

	log.Info().
		Str("operator", Operator).
		Int64("mid", mid).
		Str("senderId", rd.SenderId).
		Str("receiverId", rd.ReceiverId).
		Bool("self", action.Self).
		Msg("revoke personal succeeded")
	return nil
}

func (l *Logic) RevokeGroup(Operator string, mid int64) error {
	//查找消息
	rd, err := l.svcCtx.StoreClient.GetRecord(l.ctx, proto.Channel_ToGroup, mid)
	if err != nil {
		return err
	}
	target := rd.ReceiverId
	if rd.SenderId == Operator && time.Since(util.UnixToTime(int64(rd.CreateTime))) > l.revokeExpire {
		return model.ErrPermission
	}
	if rd.SenderId != Operator {
		//执行者
		memOpt, err := l.svcCtx.GroupClient.GetMember(l.ctx, util.ToInt64(target), Operator)
		if err != nil || memOpt == nil {
			return err
		}
		//消息所有者
		memOwn, err := l.svcCtx.GroupClient.GetMember(l.ctx, util.ToInt64(target), rd.SenderId)
		if err != nil || memOwn == nil {
			return err
		}
		switch memOpt.GroupMemberType {
		case biz.GroupMemberTypeOwner:
		case biz.GroupMemberTypeAdmin:
			if memOwn.GroupMemberType == biz.GroupMemberTypeOwner {
				return model.ErrPermission
			}
		default:
			return model.ErrPermission
		}
	}
	action := &proto.SignalRevoke{
		Mid:      mid,
		Operator: Operator,
		Self:     rd.SenderId == Operator,
	}
	body, err := xproto.Marshal(action)
	if err != nil {
		return errors.WithMessagef(err, "proto.Marshal, action=%+v", action)
	}
	if err := l.svcCtx.StoreClient.DelRecord(l.ctx, proto.Channel_ToGroup, mid); err != nil {
		return err
	}
	return l.svcCtx.AnswerClient.GroupCastSignal(l.ctx, proto.SignalType_Revoke, target, body)
}
