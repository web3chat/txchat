package logic

import (
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/types"
	xerror "github.com/web3chat/txchat/dtalk/pkg/error"
	pb "github.com/web3chat/txchat/dtalk/service/group/api"
)

func (l *GroupLogic) SetAdmin(req *types.SetAdminReq) (*types.SetAdminResp, error) {
	if _, ok := pb.GroupMemberType_name[req.MemberType]; !ok {
		return nil, xerror.NewError(xerror.ParamsError)
	}

	_, err := l.svcCtx.GroupClient.SetAdmin(l.ctx, &pb.SetAdminReq{
		GroupId:         req.Id,
		PersonId:        l.getOpe(),
		MemberId:        req.MemberId,
		GroupMemberType: pb.GroupMemberType(req.MemberType),
	})
	if err != nil {
		return nil, err
	}

	return &types.SetAdminResp{}, nil
}
