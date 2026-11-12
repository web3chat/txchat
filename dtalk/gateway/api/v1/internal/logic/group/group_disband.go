package logic

import (
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/types"
	pb "github.com/web3chat/txchat/dtalk/service/group/api"
)

func (l *GroupLogic) GroupDisband(req *types.GroupDisbandReq) (*types.GroupDisbandResp, error) {
	_, err := l.svcCtx.GroupClient.GroupDisband(l.ctx, &pb.GroupDisbandReq{
		GroupId:  req.Id,
		PersonId: l.getOpe(),
	})
	if err != nil {
		return nil, err
	}

	return &types.GroupDisbandResp{}, nil
}
