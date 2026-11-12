package service

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/web3chat/txchat/dtalk/service/record/store/burn"
	"github.com/web3chat/txchat/dtalk/service/record/store/model"
)

// BurnPrivate 收方确认焚毁：鉴权后物理删除私聊 content 与 relation。
// mid 不存在视为幂等成功。UserRecords 仅测试引用，生产同步走 DB，故不清理 Redis。
func (s *Service) BurnPrivate(ctx context.Context, operator, targetID string, mid int64) (status int32, burned bool, senderID, receiverID string, err error) {
	_ = ctx
	logger := log.With().
		Int64("mid", mid).
		Str("operatorUid", operator).
		Str("targetUid", targetID).
		Str("appId", s.cfg.AppId).
		Logger()

	content, err := s.dao.GetSpecifyRecord(mid)
	if err != nil {
		if err == model.ErrRecordNotFind {
			logger.Info().Msg("burn private mid not found, treat as idempotent success")
			return model.BurnPrivateOK, false, "", "", nil
		}
		logger.Error().Err(err).Msg("burn private get record failed")
		return 0, false, "", "", err
	}

	if !burn.IsBurnFromSource([]byte(content.Source)) {
		logger.Error().
			Str("senderId", content.SenderId).
			Str("receiverId", content.ReceiverId).
			Str("source", content.Source).
			Msg("burn private reject: not a burn message")
		return model.BurnPrivateNotBurn, false, content.SenderId, content.ReceiverId, nil
	}

	rels, err := s.dao.GetRelationsByMid(mid)
	if err != nil {
		logger.Error().Err(err).Msg("burn private get relations failed")
		return 0, false, "", "", err
	}
	if !isBurnReceiver(rels, operator, targetID) {
		logger.Error().
			Str("senderId", content.SenderId).
			Str("receiverId", content.ReceiverId).
			Int("relationCount", len(rels)).
			Msg("burn private reject: permission denied")
		return model.BurnPrivateDenied, false, content.SenderId, content.ReceiverId, nil
	}

	if err = s.dao.DeletePrivateMessageFully(mid); err != nil {
		logger.Error().Err(err).
			Str("senderId", content.SenderId).
			Str("receiverId", content.ReceiverId).
			Msg("burn private delete failed")
		return 0, false, "", "", err
	}

	logger.Info().
		Str("senderId", content.SenderId).
		Str("receiverId", content.ReceiverId).
		Bool("burned", true).
		Msg("burn private deleted")
	return model.BurnPrivateOK, true, content.SenderId, content.ReceiverId, nil
}

func isBurnReceiver(rels []*model.MsgRelation, operator, targetID string) bool {
	for _, rel := range rels {
		if rel == nil {
			continue
		}
		if rel.OwnerUid == operator && rel.OtherUid == targetID && rel.Type == model.Rev {
			return true
		}
	}
	return false
}

// classifyBurnGroupOutcome 按库状态判定群聊焚毁结果，供单测覆盖状态表。
func classifyBurnGroupOutcome(contentFound, isBurn bool, receiverID, targetID string, hasRev, hasSend bool) (status int32, shouldDeleteRev bool) {
	if !contentFound {
		return model.BurnGroupOK, false
	}
	if !isBurn {
		return model.BurnGroupNotBurn, false
	}
	if receiverID != targetID {
		return model.BurnGroupDenied, false
	}
	if hasRev {
		return model.BurnGroupOK, true
	}
	if hasSend {
		return model.BurnGroupDenied, false
	}
	return model.BurnGroupOK, false
}

func isLastGroupReceiver(operator string, recvRels []*model.MsgRelation) bool {
	for _, rel := range recvRels {
		if rel == nil {
			continue
		}
		if rel.OwnerUid != operator {
			return false
		}
	}
	return true
}

func operatorHasReceiverRow(operator string, recvRels []*model.MsgRelation) bool {
	for _, rel := range recvRels {
		if rel != nil && rel.OwnerUid == operator {
			return true
		}
	}
	return false
}

// BurnGroup 收方确认焚毁：按人删除 type=收 的 relation；最后一名收方同事务删光剩余 relation，留下 content 孤行。
func (s *Service) BurnGroup(ctx context.Context, operator, targetID string, mid int64) (status int32, burned, last bool, senderID string, err error) {
	_ = ctx
	logger := log.With().
		Int64("mid", mid).
		Str("operatorUid", operator).
		Str("targetUid", targetID).
		Str("appId", s.cfg.AppId).
		Logger()

	tx, err := s.dao.NewTx()
	if err != nil {
		logger.Error().Err(err).Msg("burn group new tx failed")
		return 0, false, false, "", err
	}
	committed := false
	defer func() {
		if !committed {
			tx.RollBack()
		}
	}()

	content, err := s.dao.GetGroupMsgContentForUpdate(tx, mid)
	if err != nil {
		if err == model.ErrRecordNotFind {
			if err = tx.Commit(); err != nil {
				logger.Error().Err(err).Msg("burn group commit missing content failed")
				return 0, false, false, "", err
			}
			committed = true
			logger.Info().
				Bool("burned", false).
				Bool("last", false).
				Msg("burn group content not found, treat as idempotent success")
			return model.BurnGroupOK, false, false, "", nil
		}
		logger.Error().Err(err).Msg("burn group lock content failed")
		return 0, false, false, "", err
	}

	isBurn := burn.IsBurnFromSource([]byte(content.Source))
	senderID = content.SenderId
	status, _ = classifyBurnGroupOutcome(true, isBurn, content.ReceiverId, targetID, false, false)
	if status == model.BurnGroupNotBurn {
		if err = tx.Commit(); err != nil {
			logger.Error().Err(err).Str("senderId", senderID).Msg("burn group commit not-burn failed")
			return 0, false, false, "", err
		}
		committed = true
		logger.Error().
			Str("senderId", senderID).
			Str("receiverId", content.ReceiverId).
			Str("source", content.Source).
			Bool("burn", false).
			Msg("burn group reject: not a burn message")
		return model.BurnGroupNotBurn, false, false, senderID, nil
	}
	if status == model.BurnGroupDenied {
		if err = tx.Commit(); err != nil {
			logger.Error().Err(err).Str("senderId", senderID).Msg("burn group commit target mismatch failed")
			return 0, false, false, "", err
		}
		committed = true
		logger.Error().
			Str("senderId", senderID).
			Str("receiverId", content.ReceiverId).
			Bool("burn", true).
			Msg("burn group reject: targetId does not match group id")
		return model.BurnGroupDenied, false, false, senderID, nil
	}

	recvRels, err := s.dao.GetGroupReceiverRelationsForUpdate(tx, mid)
	if err != nil {
		logger.Error().Err(err).
			Str("senderId", senderID).
			Bool("burn", true).
			Msg("burn group lock receiver relations failed")
		return 0, false, false, "", err
	}

	hasRev := operatorHasReceiverRow(operator, recvRels)
	hasSend := false
	if !hasRev {
		ownerRel, oerr := s.dao.GetGroupOwnerRelationForUpdate(tx, mid, operator)
		if oerr != nil {
			logger.Error().Err(oerr).
				Str("senderId", senderID).
				Bool("burn", true).
				Msg("burn group get operator relation failed")
			return 0, false, false, "", oerr
		}
		if ownerRel != nil && ownerRel.Type == model.Send {
			hasSend = true
		}
	}

	status, shouldDelete := classifyBurnGroupOutcome(true, true, content.ReceiverId, targetID, hasRev, hasSend)
	if status != model.BurnGroupOK {
		if err = tx.Commit(); err != nil {
			logger.Error().Err(err).
				Str("senderId", senderID).
				Int32("status", status).
				Msg("burn group commit denied failed")
			return 0, false, false, "", err
		}
		committed = true
		logger.Error().
			Str("senderId", senderID).
			Int("receiverCount", len(recvRels)).
			Bool("hasRev", hasRev).
			Bool("hasSend", hasSend).
			Bool("burn", true).
			Msg("burn group reject: permission denied")
		return status, false, false, senderID, nil
	}
	if !shouldDelete {
		if err = tx.Commit(); err != nil {
			logger.Error().Err(err).Str("senderId", senderID).Msg("burn group commit idempotent failed")
			return 0, false, false, "", err
		}
		committed = true
		logger.Info().
			Str("senderId", senderID).
			Bool("burned", false).
			Bool("last", false).
			Bool("burn", true).
			Msg("burn group idempotent: no receiver row")
		return model.BurnGroupOK, false, false, senderID, nil
	}

	last = isLastGroupReceiver(operator, recvRels)
	if last {
		if err = s.dao.DeleteGroupRelationsByMid(tx, mid); err != nil {
			logger.Error().Err(err).
				Str("senderId", senderID).
				Bool("burn", true).
				Bool("last", true).
				Msg("burn group delete remaining relations failed")
			return 0, false, false, "", err
		}
	} else if err = s.dao.DeleteGroupReceiverRelation(tx, mid, operator); err != nil {
		logger.Error().Err(err).
			Str("senderId", senderID).
			Bool("burn", true).
			Bool("last", false).
			Msg("burn group delete receiver relation failed")
		return 0, false, false, "", err
	}

	if err = tx.Commit(); err != nil {
		logger.Error().Err(err).
			Str("senderId", senderID).
			Bool("burn", true).
			Bool("last", last).
			Msg("burn group commit failed")
		return 0, false, false, "", err
	}
	committed = true
	logger.Info().
		Str("senderId", senderID).
		Bool("burned", true).
		Bool("last", last).
		Bool("burn", true).
		Msg("burn group deleted receiver relation")
	return model.BurnGroupOK, true, last, senderID, nil
}
