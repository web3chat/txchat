package record

import (
	"errors"

	"github.com/rs/zerolog/log"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/model"
	storemodel "github.com/web3chat/txchat/dtalk/service/record/store/model"
	"github.com/web3chat/txchat/imparse/proto"
)

func (l *Logic) RevokeBatch(operator string, req *model.RevokeBatchMsgReq) *model.RevokeBatchMsgResp {
	logIds := dedupeLogIds(req.LogIds)
	resp := &model.RevokeBatchMsgResp{
		Total:     len(logIds),
		Succeeded: make([]int64, 0, len(logIds)),
		Failed:    make([]model.RevokeBatchFailItem, 0),
	}

	channel := proto.Channel_ToUser
	if req.Type == model.Group {
		channel = proto.Channel_ToGroup
	}

	var expectedTarget string

	for _, mid := range logIds {
		rd, err := l.svcCtx.StoreClient.GetRecord(l.ctx, channel, mid)
		if err != nil {
			reason := mapRevokeError(err)
			log.Warn().
				Str("operator", operator).
				Int64("logId", mid).
				Int("type", req.Type).
				Str("reason", reason).
				Err(err).
				Msg("revoke batch get record failed")
			resp.Failed = append(resp.Failed, model.RevokeBatchFailItem{LogId: mid, Reason: reason})
			continue
		}

		if expectedTarget == "" {
			expectedTarget = rd.ReceiverId
		} else if rd.ReceiverId != expectedTarget {
			log.Warn().
				Str("operator", operator).
				Int64("logId", mid).
				Str("expectedTarget", expectedTarget).
				Str("actualTarget", rd.ReceiverId).
				Msg("revoke batch session mismatch")
			resp.Failed = append(resp.Failed, model.RevokeBatchFailItem{
				LogId:  mid,
				Reason: mapRevokeError(model.ErrSessionMismatch),
			})
			continue
		}

		switch req.Type {
		case model.Private:
			err = l.RevokePersonal(operator, mid)
		case model.Group:
			err = l.RevokeGroup(operator, mid)
		}
		if err != nil {
			reason := mapRevokeError(err)
			log.Warn().
				Str("operator", operator).
				Int64("logId", mid).
				Int("type", req.Type).
				Str("reason", reason).
				Err(err).
				Msg("revoke batch item failed")
			resp.Failed = append(resp.Failed, model.RevokeBatchFailItem{LogId: mid, Reason: reason})
			continue
		}
		resp.Succeeded = append(resp.Succeeded, mid)
	}

	resp.SuccessCount = len(resp.Succeeded)
	resp.FailedCount = len(resp.Failed)

	log.Info().
		Str("operator", operator).
		Int("type", req.Type).
		Int("total", resp.Total).
		Int("successCount", resp.SuccessCount).
		Int("failedCount", resp.FailedCount).
		Msg("revoke batch finished")

	return resp
}

func dedupeLogIds(logIds []int64) []int64 {
	seen := make(map[int64]struct{}, len(logIds))
	result := make([]int64, 0, len(logIds))
	for _, id := range logIds {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func mapRevokeError(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, model.ErrPermission):
		return "权限不足"
	case errors.Is(err, model.ErrSessionMismatch):
		return "不属于当前会话"
	case errors.Is(err, storemodel.ErrRecordNotFind):
		return "消息不存在"
	default:
		return err.Error()
	}
}
