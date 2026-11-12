package logic

import (
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/types"
	pb "github.com/web3chat/txchat/dtalk/service/group/api"
)

func (l *GroupLogic) GetGroupMemberInfo(req *types.GetGroupMemberInfoReq) (*types.GetGroupMemberInfoResp, error) {
	groupId := req.Id
	memberId := req.MemberId

	resp, err := l.svcCtx.GroupClient.GetGroupMemberInfo(l.ctx, &pb.GetGroupMemberInfoReq{
		GroupId:  groupId,
		PersonId: l.getOpe(),
		MemberId: memberId,
	})
	if err != nil {
		return nil, err
	}

	Member := NewTypesGroupMemberInfo(resp.Member)
	res := &types.GetGroupMemberInfoResp{}
	res.GroupMember = Member

	return res, nil
}
