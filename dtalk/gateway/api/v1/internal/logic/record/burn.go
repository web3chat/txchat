package record

import (
	"context"
	"strconv"
	"sync"
	"time"

	xproto "github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/model"
	xerror "github.com/web3chat/txchat/dtalk/pkg/error"
	store "github.com/web3chat/txchat/dtalk/service/record/store/api"
	storeModel "github.com/web3chat/txchat/dtalk/service/record/store/model"
	"github.com/web3chat/txchat/imparse/proto"
)

const burnDedupWindow = 2 * time.Second

type burnStore interface {
	BurnPrivate(ctx context.Context, operator, targetID string, mid int64) (*store.BurnPrivateReply, error)
	BurnGroup(ctx context.Context, operator, targetID string, mid int64) (*store.BurnGroupReply, error)
}

type burnDedupEntry struct {
	at     time.Time
	burned bool
}

var burnDedup sync.Map

func burnDedupKey(operator string, mid int64) string {
	return operator + ":" + strconv.FormatInt(mid, 10)
}

func resetBurnDedup() {
	burnDedup.Range(func(k, _ any) bool {
		burnDedup.Delete(k)
		return true
	})
}

func hitBurnDedup(key string) bool {
	v, ok := burnDedup.Load(key)
	if !ok {
		return false
	}
	e, ok := v.(burnDedupEntry)
	if !ok {
		burnDedup.Delete(key)
		return false
	}
	if time.Since(e.at) >= burnDedupWindow {
		burnDedup.Delete(key)
		return false
	}
	return e.burned
}

func rememberBurnDedup(key string, burned bool) {
	if !burned {
		return
	}
	burnDedup.Store(key, burnDedupEntry{at: time.Now(), burned: true})
}

func (l *Logic) BurnConfirm(operator, targetID string, mid int64, channelType int) (*model.BurnConfirmResp, error) {
	logger := log.With().
		Str("operatorUid", operator).
		Str("targetUid", targetID).
		Int64("mid", mid).
		Int("type", channelType).
		Logger()

	if channelType != model.Private && channelType != model.Group {
		logger.Error().Msg("burn confirm rejected: invalid type")
		return nil, xerror.NewError(xerror.ParamsError)
	}

	key := burnDedupKey(operator, mid)
	if hitBurnDedup(key) {
		logger.Info().Bool("burned", true).Msg("burn confirm dedup hit, skip duplicate request")
		return &model.BurnConfirmResp{LogId: mid, Burned: true}, nil
	}

	if channelType == model.Group {
		return l.burnConfirmGroup(operator, targetID, mid, key)
	}
	return l.burnConfirmPrivate(operator, targetID, mid, key)
}

func (l *Logic) burnConfirmPrivate(operator, targetID string, mid int64, key string) (*model.BurnConfirmResp, error) {
	lg := log.With().
		Str("operatorUid", operator).
		Str("targetUid", targetID).
		Int64("mid", mid).
		Int("type", model.Private).
		Logger()

	reply, err := l.burner.BurnPrivate(l.ctx, operator, targetID, mid)
	if err != nil {
		lg.Error().Err(err).Msg("burn confirm store rpc failed")
		return nil, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error())
	}

	switch reply.GetStatus() {
	case storeModel.BurnPrivateNotBurn:
		lg.Error().Int32("status", reply.GetStatus()).Msg("burn confirm rejected: not burn message")
		return nil, xerror.NewError(xerror.BurnNotBurnMessage)
	case storeModel.BurnPrivateDenied:
		lg.Error().Int32("status", reply.GetStatus()).Msg("burn confirm rejected: permission denied")
		return nil, xerror.NewError(xerror.BurnPermissionDenied)
	}

	rememberBurnDedup(key, reply.GetBurned())

	if reply.GetBurned() {
		if err = l.sendBurnedSignal(operator, reply.GetSenderId(), reply.GetReceiverId(), mid); err != nil {
			lg.Error().Err(err).
				Str("senderId", reply.GetSenderId()).
				Str("receiverId", reply.GetReceiverId()).
				Msg("burn confirm send signal failed")
			return nil, err
		}
	}

	lg.Info().
		Bool("burned", reply.GetBurned()).
		Str("senderId", reply.GetSenderId()).
		Str("receiverId", reply.GetReceiverId()).
		Msg("burn confirm succeeded")
	return &model.BurnConfirmResp{LogId: mid, Burned: reply.GetBurned()}, nil
}

func (l *Logic) burnConfirmGroup(operator, targetID string, mid int64, key string) (*model.BurnConfirmResp, error) {
	lg := log.With().
		Str("operatorUid", operator).
		Str("targetUid", targetID).
		Int64("mid", mid).
		Int("type", model.Group).
		Logger()

	reply, err := l.burner.BurnGroup(l.ctx, operator, targetID, mid)
	if err != nil {
		lg.Error().Err(err).Msg("burn group confirm store rpc failed")
		return nil, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error())
	}

	switch reply.GetStatus() {
	case storeModel.BurnGroupNotBurn:
		lg.Error().Int32("status", reply.GetStatus()).Msg("burn group confirm rejected: not burn message")
		return nil, xerror.NewError(xerror.BurnNotBurnMessage)
	case storeModel.BurnGroupDenied:
		lg.Error().Int32("status", reply.GetStatus()).Msg("burn group confirm rejected: permission denied")
		return nil, xerror.NewError(xerror.BurnPermissionDenied)
	}

	rememberBurnDedup(key, reply.GetBurned())

	if reply.GetBurned() {
		if err = l.sendGroupBurnedSignal(operator, reply.GetSenderId(), mid, reply.GetLast()); err != nil {
			lg.Error().Err(err).
				Str("senderId", reply.GetSenderId()).
				Bool("last", reply.GetLast()).
				Msg("burn group confirm send signal failed")
			return nil, err
		}
	}

	lg.Info().
		Bool("burned", reply.GetBurned()).
		Bool("last", reply.GetLast()).
		Str("senderId", reply.GetSenderId()).
		Msg("burn group confirm succeeded")
	return &model.BurnConfirmResp{LogId: mid, Burned: reply.GetBurned()}, nil
}

func (l *Logic) sendBurnedSignal(operator, senderID, receiverID string, mid int64) error {
	body, err := marshalRevokeSignal(operator, mid, senderID == operator, proto.RevokeReason_burned)
	if err != nil {
		return err
	}
	if senderID != "" {
		if err = l.caster.UniCastSignal(l.ctx, proto.SignalType_Revoke, senderID, body); err != nil {
			log.Error().Err(err).
				Str("operator", operator).
				Int64("mid", mid).
				Str("target", senderID).
				Msg("burn confirm unicast signal to sender failed")
			return err
		}
	}
	if receiverID != "" && receiverID != senderID {
		if err = l.caster.UniCastSignal(l.ctx, proto.SignalType_Revoke, receiverID, body); err != nil {
			log.Error().Err(err).
				Str("operator", operator).
				Int64("mid", mid).
				Str("target", receiverID).
				Msg("burn confirm unicast signal to receiver failed")
			return err
		}
	}
	return nil
}

func (l *Logic) sendGroupBurnedSignal(operator, senderID string, mid int64, last bool) error {
	body, err := marshalRevokeSignal(operator, mid, operator == senderID, proto.RevokeReason_burned)
	if err != nil {
		log.Error().Err(err).
			Str("operator", operator).
			Int64("mid", mid).
			Str("senderId", senderID).
			Bool("last", last).
			Msg("burn group marshal signal failed")
		return err
	}
	if operator != "" {
		if err = l.caster.UniCastSignal(l.ctx, proto.SignalType_Revoke, operator, body); err != nil {
			log.Error().Err(err).
				Str("operator", operator).
				Int64("mid", mid).
				Str("target", operator).
				Bool("last", last).
				Msg("burn group unicast signal to confirmer failed")
			return err
		}
	}
	if last && senderID != "" && senderID != operator {
		if err = l.caster.UniCastSignal(l.ctx, proto.SignalType_Revoke, senderID, body); err != nil {
			log.Error().Err(err).
				Str("operator", operator).
				Int64("mid", mid).
				Str("target", senderID).
				Bool("last", true).
				Msg("burn group unicast signal to sender failed")
			return err
		}
	}
	return nil
}

func marshalRevokeSignal(operator string, mid int64, self bool, reason proto.RevokeReason) ([]byte, error) {
	action := &proto.SignalRevoke{
		Mid:      mid,
		Operator: operator,
		Self:     self,
		Reason:   reason,
	}
	body, err := xproto.Marshal(action)
	if err != nil {
		return nil, errors.WithMessagef(err, "proto.Marshal, action=%+v", action)
	}
	return body, nil
}
