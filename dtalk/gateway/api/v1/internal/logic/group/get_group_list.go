package logic

import (
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/types"
	pb "github.com/web3chat/txchat/dtalk/service/group/api"
)

func (l *GroupLogic) GetGroupList(req *types.GetGroupListReq) (*types.GetGroupListResp, error) {
	personId := l.getOpe()

	resp, err := l.svcCtx.GroupClient.GetGroupList(l.ctx, &pb.GetGroupListReq{
		PersonId: personId,
	})
	if err != nil {
		return nil, err
	}

	Groups := NewTypesGroupInfos(resp.Groups)

	return &types.GetGroupListResp{
		Groups: Groups,
	}, nil
}
