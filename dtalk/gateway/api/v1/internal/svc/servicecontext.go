package svc

import (
	"sync"
	"time"

	xerror "github.com/web3chat/txchat/dtalk/pkg/error"
	"github.com/web3chat/txchat/dtalk/pkg/interceptor/trace"
	chatclean "github.com/web3chat/txchat/dtalk/service/chatclean/api"
	csapi "github.com/web3chat/txchat/dtalk/service/cs/api"
	fedcfg "github.com/web3chat/txchat/dtalk/service/federation/config"
	"github.com/web3chat/txchat/dtalk/service/federation/outbound"
	xtime "github.com/web3chat/txchat/dtalk/pkg/time"
	device "github.com/web3chat/txchat/dtalk/service/device/api"
	group "github.com/web3chat/txchat/dtalk/service/group/api"
	store "github.com/web3chat/txchat/dtalk/service/record/store/api"
	"google.golang.org/grpc"

	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/config"
	answer "github.com/web3chat/txchat/dtalk/service/record/answer/api"
)

// ServiceContext 服务上下文
type ServiceContext struct {
	m sync.RWMutex
	c config.Config

	AnswerClient    *answer.Client
	StoreClient     *store.Client
	GroupClient     *group.Client
	DeviceClient    *device.Client
	ChatCleanClient *chatclean.Client

	// CS 微服务客户端
	CSClient *csapi.Client

	FedOut *outbound.Client
}

func NewServiceContext(c config.Config) *ServiceContext {
	sc := &ServiceContext{
		c:            c,
		AnswerClient: answer.New(c.AnswerRPCClient.RegAddrs, c.AnswerRPCClient.Schema, c.AnswerRPCClient.SrvName, time.Duration(c.AnswerRPCClient.Dial)),
		StoreClient:  store.New(c.StoreRPCClient.RegAddrs, c.StoreRPCClient.Schema, c.StoreRPCClient.SrvName, time.Duration(c.StoreRPCClient.Dial)),
		DeviceClient: device.New(c.DeviceRPCClient.RegAddrs, c.DeviceRPCClient.Schema, c.DeviceRPCClient.SrvName, time.Duration(c.DeviceRPCClient.Dial)),
		GroupClient: group.New(c.GroupRPCClient.RegAddrs,
			c.GroupRPCClient.Schema,
			c.GroupRPCClient.SrvName,
			time.Duration(c.GroupRPCClient.Dial),
			grpc.WithChainUnaryInterceptor(xerror.ErrClientInterceptor, trace.UnaryClientInterceptor),
		),
		ChatCleanClient: chatclean.New(
			c.ChatCleanRPCClient.RegAddrs,
			c.ChatCleanRPCClient.Schema,
			c.ChatCleanRPCClient.SrvName,
			time.Duration(c.ChatCleanRPCClient.Dial),
		),
		CSClient: csapi.New(c.CSServiceURL),
	}
	sc.FedOut = newGatewayFederation(c)


	return sc
}

// Config 获取全局配置
func (sc *ServiceContext) Config() config.Config {
	sc.m.RLock()
	defer sc.m.RUnlock()

	return sc.c
}

func newGatewayFederation(c config.Config) *outbound.Client {
	if c.Federation == nil || !c.Federation.Enabled {
		return outbound.NewFromConfig(nil)
	}
	rpcTimeout, _ := time.ParseDuration(c.Federation.RPCTimeout)
	if rpcTimeout == 0 {
		rpcTimeout = 2 * time.Second
	}
	presTimeout, _ := time.ParseDuration(c.Federation.PresenceTimeout)
	if presTimeout == 0 {
		presTimeout = 2 * time.Second
	}
	fc := &fedcfg.Config{
		ClusterID: c.Federation.ClusterID,
		Enabled:   true,
		Presence:  &fedcfg.Presence{URL: c.Federation.PresenceURL, Timeout: xtime.Duration(presTimeout)},
		MySQL: &fedcfg.MySQL{Host: c.Federation.MySQLHost, Port: c.Federation.MySQLPort, User: c.Federation.MySQLUser, Pwd: c.Federation.MySQLPwd, DbName: c.Federation.MySQLDB},
		Outbox: &fedcfg.Outbox{RPCTimeout: xtime.Duration(rpcTimeout)},
	}
	return outbound.NewFromConfig(fc)
}
