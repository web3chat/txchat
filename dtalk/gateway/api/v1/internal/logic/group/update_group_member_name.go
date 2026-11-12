package logic

import (
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/types"
	pb "github.com/web3chat/txchat/dtalk/service/group/api"
)

func (l *GroupLogic) UpdateGroupMemberName(req *types.UpdateGroupMemberNameReq) (*types.UpdateGroupMemberNameResp, error) {
	_, err := l.svcCtx.GroupClient.UpdateGroupMemberName(l.ctx, &pb.UpdateGroupMemberNameReq{
		GroupId:    req.Id,
		PersonId:   l.getOpe(),
		MemberName: req.MemberName,
	})
	if err != nil {
		return nil, err
	}

	return &types.UpdateGroupMemberNameResp{}, nil
}
