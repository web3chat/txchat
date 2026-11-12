package logic

import (
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/types"
	"github.com/web3chat/txchat/dtalk/pkg/util"
	pb "github.com/web3chat/txchat/dtalk/service/group/api"
)

func (l *GroupLogic) ChangeOwner(req *types.ChangeOwnerReq) (*types.ChangeOwnerResp, error) {
	groupId := util.ToInt64(req.Id)
	personId := l.getOpe()
	memberId := req.MemberId

	_, err := l.svcCtx.GroupClient.ChangeOwner(l.ctx, &pb.ChangeOwnerReq{
		GroupId:  groupId,
		PersonId: personId,
		MemberId: memberId,
	})
	if err != nil {
		return nil, err
	}

	return &types.ChangeOwnerResp{}, nil
}
