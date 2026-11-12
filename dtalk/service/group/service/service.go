package service

import (
	"context"
	"fmt"
	"time"

	"github.com/web3chat/txchat/dtalk/pkg/api"

	"github.com/web3chat/txchat/dtalk/pkg/api/trace"
	"github.com/web3chat/txchat/dtalk/pkg/logger"
	"github.com/web3chat/txchat/dtalk/service/group/model/biz"

	"github.com/rs/zerolog"
	"github.com/web3chat/txchat/dtalk/pkg/naming"
	"github.com/web3chat/txchat/dtalk/pkg/net/grpc"
	idgen "github.com/web3chat/txchat/dtalk/service/generator/api"
	"github.com/web3chat/txchat/dtalk/service/group/config"
	"github.com/web3chat/txchat/dtalk/service/group/dao"
	answer "github.com/web3chat/txchat/dtalk/service/record/answer/api"
	logic "github.com/web3chat/txchat/im/api/logic/grpc"
	"google.golang.org/grpc/resolver"
)

type Service struct {
	log            zerolog.Logger
	cfg            *config.Config
	dao            *dao.Dao
	idGenRPCClient *idgen.Client
	logicClient    logic.LogicClient
	answerClient   *answer.Client
	// 新增：DAO层
	WhitelistDAO *dao.WhitelistDAO
}

var srvName = "group/srv"

func New(cfg *config.Config) *Service {
	daoInstance := dao.New(cfg)
	s := &Service{
		log:            logger.New(cfg.Env, srvName),
		cfg:            cfg,
		dao:            daoInstance,
		idGenRPCClient: idgen.New(cfg.IdGenRPCClient.RegAddrs, cfg.IdGenRPCClient.Schema, cfg.IdGenRPCClient.SrvName, time.Duration(cfg.IdGenRPCClient.Dial)),
		logicClient:    newLogicClient(cfg),
		answerClient:   answer.New(cfg.AnswerRPCClient.RegAddrs, cfg.AnswerRPCClient.Schema, cfg.AnswerRPCClient.SrvName, time.Duration(cfg.AnswerRPCClient.Dial)),
		WhitelistDAO:   dao.NewWhitelistDAO(daoInstance),
	}

	initGroupDefault(cfg.GroupInfoConfig)

	return s
}

func newLogicClient(cfg *config.Config) logic.LogicClient {
	rb := naming.NewResolver(cfg.LogicRPCClient.RegAddrs, cfg.LogicRPCClient.Schema)
	resolver.Register(rb)

	addr := fmt.Sprintf("%s:///%s", cfg.LogicRPCClient.Schema, cfg.LogicRPCClient.SrvName) // "schema://[authority]/service"
	fmt.Println("logic rpc client call addr:", addr)

	conn, err := grpc.NewGRPCConn(addr, time.Duration(cfg.LogicRPCClient.Dial))
	if err != nil {
		panic(err)
	}
	return logic.NewLogicClient(conn)
}

func (s *Service) Ping() error {
	return nil
}

func (s Service) Config() *config.Config {
	return s.cfg
}

func initGroupDefault(cfg *config.GroupDefault) {
	if cfg.GroupMaximum < 200 {
		cfg.GroupMaximum = 200
	}
	if cfg.GroupMaximum > 2000 {
		cfg.GroupMaximum = 2000
	}
	biz.GroupMaximum = cfg.GroupMaximum

	if cfg.AdminNum < 10 {
		cfg.AdminNum = 10
	}
	if cfg.AdminNum > 10 {
		cfg.AdminNum = 10
	}
	biz.AdminNum = cfg.AdminNum
}

func (s *Service) GetLog() zerolog.Logger {
	return s.log
}

func (s *Service) GetLogWithTrace(ctx context.Context) zerolog.Logger {
	logId := s.GetTrace(ctx)
	return s.log.With().Str("trace", logId).Logger()
}

func (s *Service) GetTrace(ctx context.Context) string {
	return trace.NewTraceIdWithContext(ctx)
}

func (s *Service) GetOpe(ctx context.Context) string {
	return api.NewAddrWithContext(ctx)
}
