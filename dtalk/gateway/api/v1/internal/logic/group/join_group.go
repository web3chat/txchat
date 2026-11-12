package logic

import (
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/types"
	"github.com/web3chat/txchat/dtalk/pkg/util"
	pb "github.com/web3chat/txchat/dtalk/service/group/api"
)

func (l *GroupLogic) JoinGroup(req *types.JoinGroupReq) (*types.JoinGroupResp, error) {
	groupId := req.Id

	_, err := l.svcCtx.GroupClient.InviteGroupMembers(l.ctx, &pb.InviteGroupMembersReq{
		GroupId:   groupId,
		InviterId: req.InviterId,
		MemberIds: []string{l.getOpe()},
	})
	if err != nil {
		return nil, err
	}

	return &types.JoinGroupResp{
		Id:    groupId,
		IdStr: util.ToString(groupId),
	}, nil
}
