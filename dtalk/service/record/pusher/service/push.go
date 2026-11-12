package service

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/rs/zerolog"
	"github.com/web3chat/txchat/dtalk/pkg/util"
	device "github.com/web3chat/txchat/dtalk/service/device/api"
	fedapi "github.com/web3chat/txchat/dtalk/service/federation/api"
	"github.com/web3chat/txchat/dtalk/service/federation/dao"
	offlinepush "github.com/web3chat/txchat/dtalk/service/offline-push/api"
	record "github.com/web3chat/txchat/dtalk/service/record/proto"
	"github.com/web3chat/txchat/dtalk/service/record/pusher/logH"
	"github.com/web3chat/txchat/dtalk/service/record/store/model"
	comet "github.com/web3chat/txchat/im/api/comet/grpc"
	logic "github.com/web3chat/txchat/im/api/logic/grpc"
	"github.com/web3chat/txchat/imparse/chat"
	xproto "github.com/web3chat/txchat/imparse/proto"
)

// just socket push
func (s *Service) PushUser(ctx context.Context, key, from, mid, target string, tp int32, frameType string, body []byte) error {
	log := zerolog.Ctx(ctx).With().Str("Uid", from).Str("ConnId", key).
		Str("Mid", mid).Str("Target", target).Int32("Push Type", tp).Str("Frame Type", frameType).Logger()
	err := s.pusher.UniCast(ctx, &record.PushMsg{
		AppId:     s.cfg.AppId,
		FromId:    from,
		Mid:       util.ToInt64(mid),
		Key:       key,
		Target:    target,
		Msg:       body,
		Type:      tp,
		FrameType: frameType,
	})
	if err != nil {
		log.Error().Stack().Err(err).Msg("PushUser failed")
	}
	return nil
}

// just socket push
func (s *Service) PushClient(ctx context.Context, key, from, mid, target string, tp int32, frameType string, body []byte) error {
	log := zerolog.Ctx(ctx).With().Str("Uid", from).Str("ConnId", key).
		Str("Mid", mid).Str("Target", target).Int32("Push Type", tp).Str("Frame Type", frameType).Logger()
	err := s.pusher.UniCastDevices(ctx, &record.PushMsg{
		AppId:     s.cfg.AppId,
		FromId:    from,
		Mid:       util.ToInt64(mid),
		Key:       key,
		Target:    target,
		Msg:       body,
		Type:      tp,
		FrameType: frameType,
	})
	if err != nil {
		log.Error().Stack().Err(err).Msg("PushClient failed")
	}
	return nil
}

type Pusher struct {
	s *Service
}

// getAppIdByAddress 根据地址格式确定appId
// 0x开头的地址使用dtalk-eth，其他使用dtalk
// //新增by bz
func (p *Pusher) getAppIdByAddress(address string) string {
	if len(address) >= 2 && address[0:2] == "0x" {
		return "dtalk-eth"
	}
	return "dtalk"
}

func (p *Pusher) UniCastDevices(ctx context.Context, m *record.PushMsg) error {
	keysMsg := &logic.KeysMsg{
		AppId:  m.GetAppId(),
		ToKeys: []string{m.GetTarget()},
		Msg:    m.GetMsg(),
	}

	reply, err := p.s.logicClient.PushByKeys(ctx, keysMsg)
	if err != nil {
		return err
	}

	index := comet.PushMsgReply{}
	err = proto.Unmarshal(reply.Msg, &index)
	if err != nil {
		return fmt.Errorf("unmarshal PushMsgReply failed: %v", err)
	}
	item := &logH.ConnSeqItem{
		Type:   m.GetFrameType(),
		Sender: m.GetFromId(),
		Client: m.GetKey(),
		Logs:   []int64{m.GetMid()},
	}
	for cid, seq := range index.Index {
		err := p.s.lh.Save(cid, seq, item)
		if err != nil {
			p.s.log.Warn().Err(err).Str("key", m.GetKey()).Str("from", m.GetFromId()).Str("cid", cid).Int32("seq", seq).Msg("AddConnSeqIndex failed")
		}
	}
	return nil
}

func (p *Pusher) UniCast(ctx context.Context, m *record.PushMsg) error {
	// 根据目标地址确定appId（修复appId不匹配问题）
	targetAppId := p.getAppIdByAddress(m.GetTarget())
	fromAppId := p.getAppIdByAddress(m.GetFromId())

	// 解析消息体，检查from字段
	var protoMsg comet.Proto
	var seqStr string
	var contentHash string
	var isBurn bool
	if err := proto.Unmarshal(m.GetMsg(), &protoMsg); err != nil {
		p.s.log.Error().Err(err).
			Int64("mid", m.GetMid()).
			Str("from", m.GetFromId()).
			Str("target", m.GetTarget()).
			Msg("UniCast unmarshal comet proto failed, treat as non-burn")
	} else {
		var bizProto xproto.Proto
		if err := proto.Unmarshal(protoMsg.Body, &bizProto); err != nil {
			p.s.log.Error().Err(err).
				Int64("mid", m.GetMid()).
				Str("from", m.GetFromId()).
				Str("target", m.GetTarget()).
				Msg("UniCast unmarshal biz proto failed, treat as non-burn")
		} else if bizProto.GetEventType() == xproto.Proto_common {
			var common xproto.Common
			if err := proto.Unmarshal(bizProto.GetBody(), &common); err != nil {
				p.s.log.Error().Err(err).
					Int64("mid", m.GetMid()).
					Str("from", m.GetFromId()).
					Str("target", m.GetTarget()).
					Msg("UniCast unmarshal common failed, treat as non-burn")
			} else {
				seqStr = common.GetSeq()
				isBurn = common.GetSource().GetBurn()
				contentHash = dao.ContentFingerprint(
					fromAppId, m.GetMid(), seqStr, m.GetFromId(), m.GetTarget(),
					uint32(common.GetMsgType()), common.GetDatetime(),
					string(model.ParseCommonMsg(&common)),
					string(model.ParseSource(&common)),
					string(model.ParseReference(&common)),
				)
				p.s.log.Info().
					Str("from", m.GetFromId()).
					Str("target", m.GetTarget()).
					Str("msgFrom", common.GetFrom()).
					Str("msgTarget", common.GetTarget()).
					Int64("mid", m.GetMid()).
					Bool("burn", isBurn).
					Msg("UniCast: message body from/target fields")
			}
		}
	}

	// 方案 E：PUSH2 私聊先持久化 Mirror outbox，再执行在线推送
	if isBurn {
		p.logBurnSkip(m, "mirror")
	} else if err := p.enqueueMirrorPrivateIfNeeded(ctx, m, fromAppId, seqStr, contentHash); err != nil {
		return err
	}

	// 先推送给发送者（多端同步）
	fromMidsMsg := &logic.MidsMsg{
		AppId: fromAppId,
		ToIds: []string{m.GetFromId()},
		Msg:   m.GetMsg(),
	}
	_, err := p.s.logicClient.PushByMids(ctx, fromMidsMsg)
	if err != nil {
		p.s.log.Debug().Err(err).
			Str("appId", fromMidsMsg.GetAppId()).Strs("toIds", fromMidsMsg.GetToIds()).Int("len of msg", len(fromMidsMsg.GetMsg())).
			Msg("UniCast PushByMids Failed (from)")
	}

	// 推送给目标用户（本集群）
	targetMidsMsg := &logic.MidsMsg{
		AppId: targetAppId,
		ToIds: []string{m.GetTarget()},
		Msg:   m.GetMsg(),
	}
	reply, err := p.s.logicClient.PushByMids(ctx, targetMidsMsg)
	localOnline := false
	if err != nil {
		p.s.log.Debug().Err(err).
			Str("appId", targetMidsMsg.GetAppId()).Strs("toIds", targetMidsMsg.GetToIds()).
			Str("target", m.GetTarget()).Int64("mid", m.GetMid()).
			Msg("UniCast PushByMids Failed (target)")
	} else if reply != nil && len(reply.Msg) > 0 {
		index := comet.PushMsgReply{}
		if uerr := proto.Unmarshal(reply.Msg, &index); uerr != nil {
			p.s.log.Error().Err(uerr).Int64("mid", m.GetMid()).Msg("UniCast unmarshal PushMsgReply failed")
		} else {
			localOnline = len(index.Index) > 0
			item := &logH.ConnSeqItem{
				Type:   m.GetFrameType(),
				Sender: m.GetFromId(),
				Client: m.GetKey(),
				Logs:   []int64{m.GetMid()},
			}
			for cid, seq := range index.Index {
				if serr := p.s.lh.Save(cid, seq, item); serr != nil {
					p.s.log.Warn().Err(serr).Str("key", m.GetKey()).Str("from", m.GetFromId()).Str("cid", cid).Int32("seq", seq).Msg("AddConnSeqIndex failed")
				}
			}
		}
	}

	// 本地 miss：尝试联邦投递到目标所在集群
	if !localOnline && p.s.fedOut != nil && p.s.fedOut.Enabled() {
		if isBurn {
			p.logBurnSkip(m, "forward")
		} else {
			ferr := p.s.fedOut.ForwardPrivate(ctx, targetAppId, m.GetFromId(), m.GetTarget(), m.GetMid(), seqStr, m.GetFrameType(), m.GetMsg())
			if ferr != nil {
				p.s.log.Error().Err(ferr).
					Str("from", m.GetFromId()).Str("target", m.GetTarget()).Int64("mid", m.GetMid()).
					Str("appId", targetAppId).
					Msg("UniCast federation ForwardPrivate failed")
			} else {
				p.s.log.Info().
					Str("from", m.GetFromId()).Str("target", m.GetTarget()).Int64("mid", m.GetMid()).
					Msg("UniCast federation ForwardPrivate done")
				return nil
			}
		}
	}

	// 本地与联邦都未送达：走离线推
	if !localOnline && p.s.cfg.OffPush != nil && p.s.cfg.OffPush.IsEnabled {
		p.pushOffline(m, []string{m.GetTarget()})
	}

	p.s.log.Debug().
		Str("appId", targetAppId).
		Str("from", m.GetFromId()).
		Str("target", m.GetTarget()).
		Bool("localOnline", localOnline).
		Msg("UniCast done")
	return nil
}

func (p *Pusher) enqueueMirrorPrivateIfNeeded(ctx context.Context, m *record.PushMsg, appID, seq, contentHash string) error {
	if m.GetOrigin() != record.MessageOrigin_MESSAGE_ORIGIN_PUSH2 {
		return nil
	}
	if m.GetFrameType() != string(chat.PrivateFrameType) {
		return nil
	}
	if p.s.fedOut == nil || !p.s.fedOut.Enabled() {
		return nil
	}
	if p.s.cfg.Federation != nil && p.s.cfg.Federation.Mirror != nil && !p.s.cfg.Federation.Mirror.Enabled {
		return nil
	}
	clusterID := p.s.fedOut.ClusterID()
	key := fmt.Sprintf("%s:mirror:%d", clusterID, m.GetMid())
	req := &fedapi.MirrorPrivateReq{
		SourceCluster:  clusterID,
		AppId:          appID,
		From:           m.GetFromId(),
		Target:         m.GetTarget(),
		Mid:            m.GetMid(),
		Seq:            seq,
		FrameType:      m.GetFrameType(),
		Msg:            m.GetMsg(),
		IdempotencyKey: key,
		Hop:            1,
		ContentHash:    contentHash,
	}
	if err := p.s.fedOut.EnqueueMirrorPrivate(ctx, req); err != nil {
		p.s.log.Error().Err(err).
			Str("appId", appID).Int64("mid", m.GetMid()).
			Str("from", m.GetFromId()).Str("target", m.GetTarget()).
			Str("key", key).
			Msg("enqueue MirrorPrivate outbox failed")
		return err
	}
	p.s.log.Info().
		Str("appId", appID).Int64("mid", m.GetMid()).
		Str("from", m.GetFromId()).Str("target", m.GetTarget()).
		Str("key", key).
		Msg("enqueue MirrorPrivate outbox ok")
	return nil
}

func (p *Pusher) logBurnSkip(m *record.PushMsg, skipReason string) {
	federationEnabled := p.s.fedOut != nil && p.s.fedOut.Enabled()
	clusterID := ""
	if p.s.fedOut != nil {
		clusterID = p.s.fedOut.ClusterID()
	}
	p.s.log.Error().
		Int64("mid", m.GetMid()).
		Str("from", m.GetFromId()).
		Str("target", m.GetTarget()).
		Str("clusterID", clusterID).
		Bool("federationEnabled", federationEnabled).
		Str("origin", m.GetOrigin().String()).
		Str("skipReason", skipReason).
		Bool("burn", true).
		Msg("skip burn message federation delivery")
}

func isBurnPushMsg(m *record.PushMsg) bool {
	var protoMsg comet.Proto
	if err := proto.Unmarshal(m.GetMsg(), &protoMsg); err != nil {
		return false
	}
	var bizProto xproto.Proto
	if err := proto.Unmarshal(protoMsg.Body, &bizProto); err != nil {
		return false
	}
	if bizProto.GetEventType() != xproto.Proto_common {
		return false
	}
	var common xproto.Common
	if err := proto.Unmarshal(bizProto.GetBody(), &common); err != nil {
		return false
	}
	return common.GetSource().GetBurn()
}

// shouldForwardGroup 群消息是否跨集群 ForwardGroup：burn 一律不转发。
func shouldForwardGroup(isBurn, fedEnabled bool) bool {
	return fedEnabled && !isBurn
}

func (p *Pusher) GroupCast(ctx context.Context, m *record.PushMsg) error {
	isBurn := isBurnPushMsg(m)
	if isBurn {
		p.s.log.Info().
			Int64("mid", m.GetMid()).
			Str("from", m.GetFromId()).
			Str("target", m.GetTarget()).
			Str("appId", m.GetAppId()).
			Bool("burn", true).
			Msg("group burn local GroupCast")
	}
	gMsg := &logic.GroupMsg{
		AppId: m.GetAppId(),
		Group: m.GetTarget(),
		Msg:   m.GetMsg(),
	}

	reply, err := p.s.logicClient.PushGroup(ctx, gMsg)
	if err != nil {
		p.s.log.Debug().Err(err).
			Str("appId", gMsg.GetAppId()).Str("to group", gMsg.GetGroup()).Int("len of msg", len(gMsg.GetMsg())).
			Int64("mid", m.GetMid()).
			Bool("burn", isBurn).
			Msg("GroupCast PushGroup Failed")
		return err
	}
	p.s.log.Debug().Str("appId", gMsg.AppId).Str("to group", gMsg.Group).Bool("burn", isBurn).Msg("GroupCast success")

	index := comet.PushMsgReply{}
	err = proto.Unmarshal(reply.Msg, &index)
	if err != nil {
		p.s.log.Error().Err(err).
			Int64("mid", m.GetMid()).
			Str("target", m.GetTarget()).
			Bool("burn", isBurn).
			Msg("GroupCast unmarshal PushMsgReply failed")
		return fmt.Errorf("unmarshal PushMsgReply failed: %v", err)
	}
	item := &logH.ConnSeqItem{
		Type:   m.GetFrameType(),
		Sender: m.GetFromId(),
		Client: m.GetKey(),
		Logs:   []int64{m.GetMid()},
	}
	for cid, seq := range index.Index {
		err := p.s.lh.SaveProxy(cid, seq, item)
		if err != nil {
			p.s.log.Warn().Err(err).Str("key", m.GetKey()).Str("from", m.GetFromId()).Str("cid", cid).Int32("seq", seq).Msg("AddConnSeqIndex failed")
		}
	}

	fedEnabled := p.s.fedOut != nil && p.s.fedOut.Enabled()
	if !shouldForwardGroup(isBurn, fedEnabled) {
		if isBurn && fedEnabled {
			p.logBurnSkip(m, "forward-group")
		}
		return nil
	}

	gid := util.ToInt64(m.GetTarget())
	members, merr := p.s.dao.GetMemberIds(ctx, gid)
	if merr != nil {
		p.s.log.Error().Err(merr).Str("group", m.GetTarget()).Int64("mid", m.GetMid()).Msg("GroupCast GetMemberIds failed")
		return nil
	}
	if len(members) == 0 {
		return nil
	}
	if ferr := p.s.fedOut.ForwardGroup(ctx, m.GetAppId(), m.GetFromId(), m.GetTarget(), m.GetMid(), m.GetFrameType(), m.GetMsg(), members); ferr != nil {
		p.s.log.Error().Err(ferr).Str("group", m.GetTarget()).Int64("mid", m.GetMid()).Msg("GroupCast ForwardGroup failed")
	}

	return nil
}

func (p *Pusher) pushOffline(m *record.PushMsg, toIds []string) {
	resp, err := p.s.deviceClient.GetUserAllDevices(context.TODO(), &device.GetUserAllDevicesRequest{
		Uid: m.GetFromId(),
	})
	if err != nil || resp == nil || len(resp.Devices) == 0 {
		p.s.log.Error().Err(err).Str("key", m.GetKey()).Str("from", m.GetFromId()).Msg("GetAllDevices failed")
		return
	}
	nickname := resp.Devices[0].Username

	//offline push
	for _, mid := range toIds {
		err := p.pushAllDevice(m, nickname, mid)
		if err != nil {
			continue
		}
	}
}

func (p *Pusher) pushAllDevice(m *record.PushMsg, nickname, mid string) error {
	resp, err := p.s.deviceClient.GetUserAllDevices(context.TODO(), &device.GetUserAllDevicesRequest{
		Uid: mid,
	})
	if err != nil {
		p.s.log.Error().Err(err).Str("key", m.GetKey()).Str("from", m.GetFromId()).Str("mid", mid).Msg("GetAllDevices failed")
		return err
	}
	if resp == nil {
		return nil
	}
	for _, dev := range resp.Devices {
		if dev.IsEnabled && dev.DTUid == dev.Uid {
			//需要推送
			pushMsg := &offlinepush.OffPushMsg{
				AppId:       m.GetAppId(),
				Device:      offlinepush.Device(dev.DeviceType),
				Title:       nickname,
				Content:     "[你收到一条消息]",
				Token:       dev.DeviceToken,
				ChannelType: int32(xproto.Channel_ToUser),
				Target:      m.GetFromId(),
				Timeout:     time.Now().Add(time.Minute * 7).Unix(),
			}
			b, err := proto.Marshal(pushMsg)
			if err != nil {
				p.s.log.Error().Err(err).Str("key", m.GetKey()).Str("from", m.GetFromId()).Str("appId", m.GetAppId()).Interface("toId", mid).Msg("Marshal pushMsg failed")
				continue
			}
			err = p.s.dao.PublishOfflineMsg(context.TODO(), m.GetKey(), b)
			if err != nil {
				p.s.log.Error().Err(err).Str("key", m.GetKey()).Str("from", m.GetFromId()).Str("appId", m.GetAppId()).Interface("toId", mid).Msg("PublishOfflineMsg failed")
			}
		}
	}
	return nil
}
