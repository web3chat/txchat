package dao

import (
	"context"
	"time"

	groupApi "github.com/web3chat/txchat/dtalk/service/group/api"
)

func (d *Dao) GetAllJoinedGroups(ctx context.Context, uid string) (groups []int64, err error) {
	var (
		req   groupApi.GetGroupIdsRequest
		reply *groupApi.GetGroupIdsReply
	)
	req.MemberId = uid
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Second*3))
	defer cancel()
	reply, err = d.groupRPCClient.GetGroupIds(ctx, &req)
	if err != nil {
		return
	}

	return reply.GroupIds, nil
}

func (d *Dao) GetGroupSession(cid string, seq int32) (session string, err error) {
	//TODO call logic get log mark by connect seq
	return "", nil
}

func (d *Dao) GetMemberIds(ctx context.Context, groupId int64) ([]string, error) {
	ctx, cancel := context.WithDeadline(ctx, time.Now().Add(time.Second*3))
	defer cancel()
	reply, err := d.groupRPCClient.GetMemberIds(ctx, &groupApi.GetMemberIdsRequest{GroupId: groupId})
	if err != nil {
		d.log.Error().Err(err).Int64("groupId", groupId).Msg("GetMemberIds failed")
		return nil, err
	}
	if reply == nil {
		return nil, nil
	}
	return reply.GetMemberIds(), nil
}
