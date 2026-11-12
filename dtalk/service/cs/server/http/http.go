package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/inconshreveable/log15"
	"github.com/web3chat/txchat/dtalk/pkg/api"
	"github.com/web3chat/txchat/dtalk/service/cs/service"
)

var (
	svc *service.Service
	log = log15.New("module", "cs/http")
)

func Init(s *service.Service) *http.Server {
	addr := s.Config().Server.Addr
	engine := Default()
	svc = s
	SetupEngine(engine)

	srv := &http.Server{
		Addr:    addr,
		Handler: engine,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("cs http start error", "err", err)
			panic(err)
		}
	}()
	return srv
}

func Default() *gin.Engine {
	router := gin.New()
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: api.Chat33GinLogFormatter,
		// /cs/decrypt 随每条入站消息高频调用，跳过 access 行避免刷盘
		SkipPaths: []string{"/cs/decrypt"},
	}))
	router.Use(gin.Recovery())
	return router
}

func SetupEngine(e *gin.Engine) *gin.Engine {
	cs := e.Group("/cs", api.RespMiddleWare())
	{
		// 公开接口
		cs.POST("/login", Login)
		cs.POST("/agents", GetAgents)

		// 需要 session 认证的接口
		authed := cs.Group("", SessionAuthMiddleware())
		{
			authed.POST("/agent/info", GetAgentInfo)
			authed.POST("/agent/status", AgentUpdateStatus)
			authed.POST("/token/refresh", RefreshFZMToken)
			authed.POST("/chain/ping", ChainPing)
			authed.POST("/chain/bind-comet", BindComet)
			authed.POST("/encrypt", EncryptMsg)
			authed.POST("/decrypt", DecryptMsg)
			authed.POST("/user-pubkey", GetUserPubKey)
			authed.POST("/register-pubkey", RegisterPubKeyOnChain)
			authed.POST("/verify-chain-pubkey", VerifyChainPubKey)
		}
	}
	return e
}
