package record

import (
	"time"

	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/model"
	xerror "github.com/web3chat/txchat/dtalk/pkg/error"
	chatcleanpb "github.com/web3chat/txchat/dtalk/service/chatclean/api"
)

// normalizeUidPair 保证 uid_a <= uid_b（与数据库约束一致）
func normalizeUidPair(uid1, uid2 string) (string, string) {
	if uid1 <= uid2 {
		return uid1, uid2
	}
	return uid2, uid1
}

// dateOptionToRange 将 DateOption 转换为 Unix ms 时间范围
func dateOptionToRange(opt model.DateOption, customStart, customEnd int64) (int64, int64) {
	now := time.Now().UnixMilli()
	ms := func(d time.Duration) int64 { return int64(d / time.Millisecond) }
	switch opt {
	case model.DateOptionCustom:
		return customStart, customEnd
	case model.DateOptionLast1Day:
		return now - ms(24*time.Hour), now
	case model.DateOptionLast3Days:
		return now - ms(3*24*time.Hour), now
	case model.DateOptionLast7Days:
		return now - ms(7*24*time.Hour), now
	case model.DateOptionLast30Days:
		return now - ms(30*24*time.Hour), now
	case model.DateOptionLast90Days:
		return now - ms(90*24*time.Hour), now
	case model.DateOptionLast180Days:
		return now - ms(180*24*time.Hour), now
	case model.DateOptionLast365Days:
		return now - ms(365*24*time.Hour), now
	case model.DateOptionAllTime:
		return 0, now
	default:
		return 0, now
	}
}

// CleanPrivateChat 通过 chatclean gRPC 服务删除私聊记录
func (l *Logic) CleanPrivateChat(req *model.CleanPrivateChatReq) (*model.CleanPrivateChatResp, error) {
	if req.TargetID == "" {
		return nil, xerror.NewError(xerror.ParamsError).SetExtMessage("target_id is required")
	}
	if req.DateOpt == model.DateOptionCustom {
		if req.StartTime <= 0 || req.EndTime <= 0 || req.StartTime >= req.EndTime {
			return nil, xerror.NewError(xerror.ParamsError).SetExtMessage("custom date range invalid")
		}
	}

	startTime, endTime := dateOptionToRange(req.DateOpt, req.StartTime, req.EndTime)

	deleted, err := l.svcCtx.ChatCleanClient.CleanPrivateChat(
		l.ctx,
		req.OperatorID, req.TargetID,
		chatcleanpb.DateOption(req.DateOpt),
		startTime, endTime,
	)
	if err != nil {
		return nil, xerror.NewError(xerror.RPCFailed).SetExtMessage(err.Error())
	}

	return &model.CleanPrivateChatResp{DeletedCount: deleted}, nil
}

// SetAutoDeleteConfig 通过 chatclean gRPC 服务保存自动删除配置
func (l *Logic) SetAutoDeleteConfig(req *model.SetAutoDeleteConfigReq) (*model.AutoDeleteConfigResp, error) {
	if req.TargetID == "" {
		return nil, xerror.NewError(xerror.ParamsError).SetExtMessage("target_id is required")
	}

	reply, err := l.svcCtx.ChatCleanClient.SetAutoDeleteConfig(
		l.ctx,
		req.OperatorID, req.TargetID,
		chatcleanpb.AutoDeleteOption(req.Option),
	)
	if err != nil {
		return nil, xerror.NewError(xerror.RPCFailed).SetExtMessage(err.Error())
	}

	return &model.AutoDeleteConfigResp{
		Option:    model.AutoDeleteOption(reply.Option),
		UpdatedBy: reply.UpdatedBy,
		UpdatedAt: reply.UpdatedAt,
	}, nil
}

// GetAutoDeleteConfig 通过 chatclean gRPC 服务查询自动删除配置
func (l *Logic) GetAutoDeleteConfig(req *model.GetAutoDeleteConfigReq) (*model.AutoDeleteConfigResp, error) {
	if req.TargetID == "" {
		return nil, xerror.NewError(xerror.ParamsError).SetExtMessage("target_id is required")
	}

	reply, err := l.svcCtx.ChatCleanClient.GetAutoDeleteConfig(
		l.ctx,
		req.OperatorID, req.TargetID,
	)
	if err != nil {
		return nil, xerror.NewError(xerror.RPCFailed).SetExtMessage(err.Error())
	}

	return &model.AutoDeleteConfigResp{
		Option:    model.AutoDeleteOption(reply.Option),
		UpdatedBy: reply.UpdatedBy,
		UpdatedAt: reply.UpdatedAt,
	}, nil
}
