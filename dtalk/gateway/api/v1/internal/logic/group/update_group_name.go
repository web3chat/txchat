package logic

import (
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/types"
	pb "github.com/web3chat/txchat/dtalk/service/group/api"
)

func (l *GroupLogic) UpdateGroupName(req *types.UpdateGroupNameReq) (*types.UpdateGroupNameResp, error) {
	_, err := l.svcCtx.GroupClient.UpdateGroupName(l.ctx, &pb.UpdateGroupNameReq{
		GroupId:    req.Id,
		PersonId:   l.getOpe(),
		Name:       req.Name,
		PublicName: req.PublicName,
	})
	if err != nil {
		return nil, err
	}

	return &types.UpdateGroupNameResp{}, nil
}
