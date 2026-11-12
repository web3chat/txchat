package api

import (
	"context"
	"fmt"
	"time"

	"github.com/web3chat/txchat/dtalk/pkg/naming"
	xgrpc "github.com/web3chat/txchat/dtalk/pkg/net/grpc"
	"google.golang.org/grpc/resolver"
)

// Client 是 ChatClean gRPC 服务的客户端封装
type Client struct {
	client ChatCleanClient
}

// New 创建 ChatClean gRPC 客户端，通过 etcd 做服务发现
func New(etcdAddr, schema, srvName string, dial time.Duration) *Client {
	rb := naming.NewResolver(etcdAddr, schema)
	resolver.Register(rb)

	addr := fmt.Sprintf("%s:///%s", schema, srvName)
	conn, err := xgrpc.NewGRPCConn(addr, dial)
	if err != nil {
		panic(err)
	}
	return &Client{client: NewChatCleanClient(conn)}
}

// CleanPrivateChat 手动删除私聊记录
func (c *Client) CleanPrivateChat(ctx context.Context, operatorID, targetID string, dateOpt DateOption, startTime, endTime int64) (int64, error) {
	reply, err := c.client.CleanPrivateChat(ctx, &CleanPrivateChatReq{
		OperatorId: operatorID,
		TargetId:   targetID,
		DateOpt:    dateOpt,
		StartTime:  startTime,
		EndTime:    endTime,
	})
	if err != nil {
		return 0, err
	}
	return reply.DeletedCount, nil
}

// SetAutoDeleteConfig 设置自动删除配置
func (c *Client) SetAutoDeleteConfig(ctx context.Context, operatorID, targetID string, option AutoDeleteOption) (*SetAutoDeleteConfigReply, error) {
	return c.client.SetAutoDeleteConfig(ctx, &SetAutoDeleteConfigReq{
		OperatorId: operatorID,
		TargetId:   targetID,
		Option:     option,
	})
}

// GetAutoDeleteConfig 查询自动删除配置
func (c *Client) GetAutoDeleteConfig(ctx context.Context, operatorID, targetID string) (*GetAutoDeleteConfigReply, error) {
	return c.client.GetAutoDeleteConfig(ctx, &GetAutoDeleteConfigReq{
		OperatorId: operatorID,
		TargetId:   targetID,
	})
}
