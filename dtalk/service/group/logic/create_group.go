package logic

import (
	"context"
	"fmt"

	pb "github.com/web3chat/txchat/dtalk/service/group/api"
	"github.com/web3chat/txchat/dtalk/service/group/model"
	"github.com/web3chat/txchat/dtalk/service/group/model/biz"
	"github.com/web3chat/txchat/dtalk/service/group/service"
)

type CreateGroupLogic struct {
	ctx context.Context
	svc *service.Service
}

func NewCreateGroupLogic(ctx context.Context, svc *service.Service) *CreateGroupLogic {
	return &CreateGroupLogic{
		ctx: ctx,
		svc: svc,
	}
}

// CreateGroup 创建群聊
func (l *CreateGroupLogic) CreateGroup(req *pb.CreateGroupReq) (*pb.CreateGroupResp, error) {
	var err error

	// 从请求中获取地址（这里需要根据实际业务逻辑获取用户地址）
	addr := l.getAddressFromRequest(req)
	if addr == "" {
		return nil, fmt.Errorf("无法获取用户地址: %w", err)
	}

	// 验证地址格式
	addressHelper := NewAddressHelper()
	if err := addressHelper.ValidateAddress(addr); err != nil {
		return nil, fmt.Errorf("地址格式无效: %w", err)
	}

	whitelistInfo, err := l.resolveWhitelistForCreateGroup(addr)
	if err != nil {
		log := l.svc.GetLogWithTrace(l.ctx)
		log.Error().Err(err).Str("address", addr).Msg("create group whitelist policy failed")
		return nil, err
	}

	// 检查已创建群聊数量是否超过白名单限制
	if whitelistInfo.MaxGroups > 0 {
		currentGroupCount, err := l.svc.GetGroupCountByOwnerId(l.ctx, addr)
		if err != nil {
			return nil, fmt.Errorf("查询已创建群聊数量失败: %w", err)
		}
		if currentGroupCount >= int64(whitelistInfo.MaxGroups) {
			fmt.Println(fmt.Errorf("已创建群聊数量超过白名单限制，当前已创建 %d 个，最大允许 %d 个", currentGroupCount, whitelistInfo.MaxGroups))
			return nil, fmt.Errorf("已创建群聊数量超过白名单限制，当前已创建 %d 个，最大允许 %d 个", currentGroupCount, whitelistInfo.MaxGroups)
		}
	}

	// 检查群成员数量是否超过白名单限制
	if whitelistInfo.MaxMembers > 0 && len(req.Members)+1 > int(whitelistInfo.MaxMembers) {
		return nil, fmt.Errorf("群成员数量超过白名单限制，最大允许 %d 人", whitelistInfo.MaxMembers)
	}

	req.Owner.Id, err = FilteredMemberId(req.Owner.Id)
	if err != nil {
		return nil, err
	}
	if whitelistInfo.MaxMembers > biz.GroupMaximum {
		whitelistInfo.MaxMembers = biz.GroupMaximum
	}

	group := &biz.GroupInfo{
		GroupName:    req.Name,
		GroupType:    int32(req.GroupType),
		GroupMaximum: whitelistInfo.MaxMembers,
	}

	owner := &biz.GroupMember{
		GroupMemberId:   req.Owner.Id,
		GroupMemberName: req.Owner.Name,
	}

	members := make([]*biz.GroupMember, 0, len(req.Members))
	for _, member := range req.Members {
		if member.Id == owner.GroupMemberId {
			continue
		}

		member.Id, err = FilteredMemberId(member.Id)
		if err != nil {
			return nil, err
		}

		members = append(members, &biz.GroupMember{
			GroupMemberId:   member.Id,
			GroupMemberName: member.Name,
		})
	}

	groupId, err := l.svc.CreateGroup(l.ctx, group, owner, members)
	if err != nil {
		return nil, err
	}

	return &pb.CreateGroupResp{
		GroupId: groupId,
	}, nil
}

// resolveWhitelistForCreateGroup 根据配置决定是否走白名单；关闭时使用全局 GroupMaximum，不限制建群数量。
func (l *CreateGroupLogic) resolveWhitelistForCreateGroup(addr string) (*model.CheckWhitelistResp, error) {
	if !l.isCreateGroupWhitelistEnabled() {
		log := l.svc.GetLogWithTrace(l.ctx)
		log.Info().Str("address", addr).Int32("groupMaximum", biz.GroupMaximum).
			Msg("create group whitelist disabled, skip DB whitelist check")
		return &model.CheckWhitelistResp{
			MaxMembers: biz.GroupMaximum,
			MaxGroups:  0,
			Address:    addr,
		}, nil
	}
	return l.checkCreateGroupPermissionWithLimit(addr)
}

func (l *CreateGroupLogic) isCreateGroupWhitelistEnabled() bool {
	cfg := l.svc.Config()
	if cfg == nil || cfg.GroupInfoConfig == nil {
		return false
	}
	if cfg.GroupInfoConfig.EnableCreateGroupWhitelist == nil {
		return false
	}
	return *cfg.GroupInfoConfig.EnableCreateGroupWhitelist
}

// checkCreateGroupPermissionWithLimit 检查创建群组白名单及白名单内人数/建群数配额
func (l *CreateGroupLogic) checkCreateGroupPermissionWithLimit(addr string) (*model.CheckWhitelistResp, error) {
	// 检查地址是否在白名单中，并获取人数上限
	whitelistInfo, err := l.svc.WhitelistDAO.CheckWhitelistWithLimit(l.ctx, addr)
	if err != nil {
		return nil, err
	}

	if !whitelistInfo.IsInWhitelist {
		return nil, model.ErrGroupCreatePermission
	}

	return whitelistInfo, nil
}

// getAddressFromRequest 从请求中获取地址
func (l *CreateGroupLogic) getAddressFromRequest(req *pb.CreateGroupReq) string {
	// 这里可以根据实际业务逻辑来获取地址
	// 例如从 Owner.Id 中解析，或者从其他字段中获取
	return req.Owner.Id
}
