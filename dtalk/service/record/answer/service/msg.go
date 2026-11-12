package service

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/rs/zerolog/log"
	"github.com/web3chat/txchat/dtalk/pkg/util"
	answerapi "github.com/web3chat/txchat/dtalk/service/record/answer/api"
	"github.com/web3chat/txchat/dtalk/service/record/answer/dao"
	"github.com/web3chat/txchat/dtalk/service/record/answer/model"
	record "github.com/web3chat/txchat/dtalk/service/record/proto"
	logic "github.com/web3chat/txchat/im/api/logic/grpc"
	"github.com/web3chat/txchat/imparse"
	"github.com/web3chat/txchat/imparse/chat"
	xproto "github.com/web3chat/txchat/imparse/proto"
)

// CtxKeyMsgOrigin 透传 PushCommonMsg.origin 到 Transport。
const CtxKeyMsgOrigin = "answer-msg-origin"

func originFromCtx(ctx context.Context) record.MessageOrigin {
	if v := ctx.Value(CtxKeyMsgOrigin); v != nil {
		switch o := v.(type) {
		case answerapi.MessageOrigin:
			return record.MessageOrigin(o)
		case record.MessageOrigin:
			return o
		}
	}
	return record.MessageOrigin_MESSAGE_ORIGIN_UNSPECIFIED
}

type DB struct {
	dao *dao.Dao
}

func (d *DB) GetMsg(ctx context.Context, from, msgId string) (*imparse.MsgIndex, error) {
	r, err := d.dao.GetRecordSeqIndex(from, msgId)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, nil
	}
	return &imparse.MsgIndex{
		Mid:        r.Mid,
		Seq:        r.Seq,
		SenderId:   r.SenderId,
		CreateTime: r.CreateTime,
	}, nil
}

func (d *DB) AddMsg(ctx context.Context, uid string, m *imparse.MsgIndex) error {
	return d.dao.AddRecordSeqIndex(uid, &model.MsgIndex{
		Mid:        m.Mid,
		Seq:        m.Seq,
		SenderId:   m.SenderId,
		CreateTime: m.CreateTime,
	})
}

func (d *DB) GetMid(ctx context.Context) (id int64, err error) {
	return d.dao.GetMid(ctx)
}

func (d *DB) GetFilters() map[imparse.FrameType][]imparse.Filter {
	//filters
	return map[imparse.FrameType][]imparse.Filter{
		chat.GroupFrameType: []imparse.Filter{
			func(ctx context.Context, frame imparse.Frame) error {
				f := frame.(*chat.GroupFrame)
				//判断群聊拦截
				if f.GetMsgType() != xproto.MsgType_Notice {
					if ok, err := d.dao.CheckInGroup(ctx, f.GetFrom(), util.ToInt64(f.GetTarget())); !ok {
						if err != nil {
							return err
						}
						return model.ErrGroupMemberNotExists
					}
				}
				return nil
			},
		},
	}
}

// send msg callback
type Exec struct {
	appId       string
	dao         *dao.Dao
	logicClient logic.LogicClient
}

func (e *Exec) Transport(ctx context.Context, mid int64, key, from, target string, ch imparse.Channel, frameType imparse.FrameType, data []byte) error {

	fmt.Println("Transport key", key)
	fmt.Println("Transport from", from)
	fmt.Println("Transport target", target)
	fmt.Println("Transport mid", mid)

	// 动态 appId：优先从 ctx 读取
	appId := e.appId
	if v := ctx.Value("answer-app-id"); v != nil {
		if s, ok := v.(string); ok && s != "" {
			appId = s
		}
	}
	pushMsg := &record.PushMsg{
		AppId:     appId,
		FromId:    from,
		Mid:       mid,
		Key:       key,
		Target:    target,
		Msg:       data,
		Type:      int32(ch),
		FrameType: string(frameType),
		Origin:    originFromCtx(ctx),
	}
	b, err := proto.Marshal(pushMsg)
	if err != nil {
		return err
	}
	return e.dao.PublishToSendWithAppId(ctx, appId, from, b)
}

func (e *Exec) RevAck(ctx context.Context, id int64, keys []string, data []byte) error {
	// 动态 appId：优先从 ctx 读取
	appId := e.appId
	if v := ctx.Value("answer-app-id"); v != nil {
		if s, ok := v.(string); ok && s != "" {
			appId = s
		}
	}
	log.Info().
		Str("appId", appId).
		Int64("id", id).
		Strs("keys", keys).
		Int("dataLen", len(data)).
		Msg("RevAck called")

	keysMsg := &logic.KeysMsg{
		AppId:  appId,
		ToKeys: keys,
		Msg:    data,
	}

	reply, err := e.logicClient.PushByKeys(ctx, keysMsg)
	if err != nil {
		log.Error().Err(err).
			Str("appId", appId).
			Int64("id", id).
			Strs("keys", keys).
			Msg("RevAck PushByKeys failed")
		return err
	}

	if reply != nil {
		log.Info().
			Str("appId", appId).
			Int64("id", id).
			Strs("keys", keys).
			Msg("RevAck PushByKeys success")
	} else {
		log.Warn().
			Str("appId", appId).
			Int64("id", id).
			Strs("keys", keys).
			Msg("RevAck PushByKeys returned nil reply")
	}

	return nil
}

// send msg callback
type withoutAckExec struct {
	appId       string
	dao         *dao.Dao
	logicClient logic.LogicClient
}

func (e *withoutAckExec) Transport(ctx context.Context, mid int64, key, from, target string, ch imparse.Channel, frameType imparse.FrameType, data []byte) error {
	// 动态 appId：优先从 ctx 读取
	appId := e.appId
	if v := ctx.Value("answer-app-id"); v != nil {
		if s, ok := v.(string); ok && s != "" {
			appId = s
		}
	}
	pushMsg := &record.PushMsg{
		AppId:     appId,
		FromId:    from,
		Mid:       mid,
		Key:       key,
		Target:    target,
		Msg:       data,
		Type:      int32(ch),
		FrameType: string(frameType),
		Origin:    originFromCtx(ctx),
	}
	b, err := proto.Marshal(pushMsg)
	if err != nil {
		return err
	}
	return e.dao.PublishToSendWithAppId(ctx, appId, from, b)
}

func (e *withoutAckExec) RevAck(ctx context.Context, id int64, keys []string, data []byte) error {
	return nil
}
