package service

import (
	"context"
	"strconv"

	"github.com/web3chat/txchat/dtalk/service/record/store/model"
	xproto "github.com/web3chat/txchat/imparse/proto"
)

// groupContentNewlyInserted 判断 INSERT ... ON DUPLICATE KEY UPDATE mid=mid 是否写入了新正文。
// 新插入 RowsAffected=1；重复键且 UPDATE 未改值时为 0 或 2。
func groupContentNewlyInserted(rowsAffected int64) bool {
	return rowsAffected == 1
}

func (s *Service) AppendGroupMsg(isSelfRead bool, p *xproto.Common) error {
	//获取所有群成员
	members, err := s.dao.AllGroupMembers(context.TODO(), p.Target)
	if err != nil {
		s.log.Error().Err(err).
			Int64("mid", p.GetMid()).
			Str("from", p.GetFrom()).
			Str("target", p.GetTarget()).
			Str("appId", s.cfg.AppId).
			Msg("AppendGroupMsg AllGroupMembers failed")
		return err
	}
	var selfRead uint8 = model.Received
	if !isSelfRead {
		selfRead = model.UnReceive
	}
	var msgRelate = make([]*model.MsgRelation, len(members))
	for i, member := range members {
		var item *model.MsgRelation
		if member == p.From {
			//发送者
			item = &model.MsgRelation{
				Mid:        strconv.FormatInt(p.Mid, 10),
				OwnerUid:   p.From,
				OtherUid:   p.Target,
				Type:       model.Send,
				State:      selfRead,
				CreateTime: p.Datetime,
			}
		} else {
			item = &model.MsgRelation{
				Mid:        strconv.FormatInt(p.Mid, 10),
				OwnerUid:   member,
				OtherUid:   p.Target,
				Type:       model.Rev,
				State:      model.UnReceive,
				CreateTime: p.Datetime,
			}
		}
		msgRelate[i] = item
	}
	tx, err := s.dao.NewTx()
	if err != nil {
		s.log.Error().Err(err).
			Int64("mid", p.GetMid()).
			Str("from", p.GetFrom()).
			Str("target", p.GetTarget()).
			Str("appId", s.cfg.AppId).
			Msg("AppendGroupMsg NewTx failed")
		return err
	}
	n, _, err := s.dao.AppendGroupMsgContent(tx, &model.MsgContent{
		Mid:        strconv.FormatInt(p.Mid, 10),
		Seq:        p.Seq,
		SenderId:   p.From,
		ReceiverId: p.Target,
		MsgType:    uint32(p.MsgType),
		Content:    string(model.ParseCommonMsg(p)),
		CreateTime: p.Datetime,
		Source:     string(model.ParseSource(p)),
		Reference:  string(model.ParseReference(p)),
	})
	if err != nil {
		tx.RollBack()
		s.log.Error().Err(err).
			Int64("mid", p.GetMid()).
			Str("from", p.GetFrom()).
			Str("target", p.GetTarget()).
			Str("appId", s.cfg.AppId).
			Msg("AppendGroupMsg AppendGroupMsgContent failed")
		return err
	}
	if !groupContentNewlyInserted(n) {
		s.log.Info().
			Int64("mid", p.GetMid()).
			Str("from", p.GetFrom()).
			Str("target", p.GetTarget()).
			Str("appId", s.cfg.AppId).
			Int64("rowsAffected", n).
			Bool("burn", p.GetSource().GetBurn()).
			Msg("group content already exists, skip relation insert")
		if err = tx.Commit(); err != nil {
			tx.RollBack()
			s.log.Error().Err(err).
				Int64("mid", p.GetMid()).
				Str("from", p.GetFrom()).
				Str("target", p.GetTarget()).
				Str("appId", s.cfg.AppId).
				Msg("AppendGroupMsg commit skip relation failed")
			return err
		}
		return nil
	}
	_, _, err = s.dao.AppendGroupMsgRelation(tx, msgRelate)
	if err != nil {
		tx.RollBack()
		s.log.Error().Err(err).
			Int64("mid", p.GetMid()).
			Str("from", p.GetFrom()).
			Str("target", p.GetTarget()).
			Str("appId", s.cfg.AppId).
			Int("relationCount", len(msgRelate)).
			Msg("AppendGroupMsg AppendGroupMsgRelation failed")
		return err
	}
	err = tx.Commit()
	if err != nil {
		tx.RollBack()
		s.log.Error().Err(err).
			Int64("mid", p.GetMid()).
			Str("from", p.GetFrom()).
			Str("target", p.GetTarget()).
			Str("appId", s.cfg.AppId).
			Msg("AppendGroupMsg commit failed")
		return err
	}
	return nil
}

func (s *Service) StoreGroupMsg(deviceType xproto.Device, pro *xproto.Common) error {
	isBurn := pro != nil && pro.GetSource() != nil && pro.GetSource().GetBurn()
	if isBurn {
		s.log.Info().
			Int64("mid", pro.GetMid()).
			Str("from", pro.GetFrom()).
			Str("target", pro.GetTarget()).
			Str("appId", s.cfg.AppId).
			Bool("burn", true).
			Msg("store group burn message")
	}
	isSelfRead := false
	if deviceType == xproto.Device_IOS || deviceType == xproto.Device_Android {
		isSelfRead = true
	}
	err := s.AppendGroupMsg(isSelfRead, pro)
	if err != nil {
		s.log.Error().Err(err).
			Int64("mid", pro.GetMid()).
			Str("from", pro.GetFrom()).
			Str("target", pro.GetTarget()).
			Str("appId", s.cfg.AppId).
			Bool("burn", isBurn).
			Msg("AppendGroupMsg failed")
		return model.ErrConsumeRedo
	}
	return nil
}
