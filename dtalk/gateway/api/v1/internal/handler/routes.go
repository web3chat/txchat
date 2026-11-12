package handler

import (
	"net/http"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	zlog "github.com/rs/zerolog/log"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/gin-swagger/swaggerFiles"
	_ "github.com/web3chat/txchat/dtalk/gateway/api/v1/docs"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/handler/account"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/handler/cs"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/handler/group"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/handler/modules"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/handler/record"
	test "github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/handler/test"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/svc"
	"github.com/web3chat/txchat/dtalk/pkg/api"
	"github.com/web3chat/txchat/dtalk/pkg/api/logger"
	"github.com/web3chat/txchat/dtalk/pkg/api/trace"
	xlog "github.com/web3chat/txchat/dtalk/pkg/logger"
	otrace "github.com/txchat/im-pkg/trace"
)

var (
	serverCtx *svc.ServiceContext
	log       = zlog.Logger
)

func Init(ctx *svc.ServiceContext) *http.Server {
	serverCtx = ctx
	addr := serverCtx.Config().Server.Addr
	engine := Default()
	SetupEngine(engine)
	SetupGroupRoutes(engine)
	SetupResourceRoutes(engine)
	pprof.Register(engine)

	srv := &http.Server{
		Addr:    addr,
		Handler: engine,
	}
	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("engineInner.Start() err")
			panic(err)
		}
	}()
	return srv
}

// Default returns an Engine instance with the Logger and Recovery middleware already attached.
func Default() *gin.Engine {
	router := gin.New()
	// LoggerWithFormatter middleware will write the logs to gin.DefaultWriter
	// By default gin.DefaultWriter = os.Stdout
	router.Use(gin.LoggerWithFormatter(api.Chat33GinLogFormatter))
	router.Use(gin.Recovery())
	return router
}

// @title           即时通讯系统后端接口
// @version         1.0
// @description
// @termsOfService
// @contact.name
// @contact.url
// @contact.email
// @schemes   https
// @host      localhost:8080
// @BasePath  /api/v1
func SetupEngine(e *gin.Engine) *gin.Engine {
	root := e.Group("/", otrace.OpenTracingServerMiddleWare(), api.RespMiddleWare())
	root.POST("/test", test.GetHelloWord(serverCtx))

	userRoute := root.Group("/user")
	//获取服务器列表

	userRoute.Use(api.AuthMiddleWare())
	{
		userRoute.POST("/login", account.AddressLogin(serverCtx))
	}
	// 返回 ETH 类型地址的登录接口，路径统一为 /eth/login
	ethRoute := root.Group("/eth")
	ethRoute.Use(api.AuthMiddleWare4EthAddress())
	{
		ethRoute.POST("/login", account.AddressLogin(serverCtx))
	}

	app := root.Group("/app")
	{
		modulesRoute := app.Group("/modules")
		//获取模块启用状态
		modulesRoute.POST("/all", modules.GetModulesHandler(serverCtx))

		recordRoute := app.Group("/record")
		{
			recordRoute.POST("/revoke", api.AuthMiddleWare(), record.RevokeHandler(serverCtx))
			recordRoute.POST("/revoke-batch", api.AuthMiddleWare(), record.RevokeBatchHandler(serverCtx))
			recordRoute.POST("/burn-confirm", api.AuthMiddleWare(), record.BurnConfirmHandler(serverCtx))
			recordRoute.POST("/focus", api.AuthMiddleWare(), record.FocusHandler(serverCtx))
			recordRoute.POST("/sync-record", api.AuthMiddleWare(), record.SyncRecords(serverCtx))

			// 聊天记录清理
			recordRoute.POST("/clean-private", api.AuthMiddleWare(), record.CleanPrivateChatHandler(serverCtx))
			recordRoute.POST("/auto-delete/set", api.AuthMiddleWare(), record.SetAutoDeleteConfigHandler(serverCtx))
			recordRoute.POST("/auto-delete/get", api.AuthMiddleWare(), record.GetAutoDeleteConfigHandler(serverCtx))
		}
	}

	recordRoute := root.Group("/record")
	recordRoute.Use(api.AuthMiddleWare())
	{
		recordRoute.POST("/push", record.PushToUid(serverCtx))
		recordRoute.POST("/push2", record.PushToUid2(serverCtx))
	}

	store := root.Group("/store/app")
	store.Use(api.AuthMiddleWare())
	{
		store.POST("/pri-chat-record", record.GetPriRecords(serverCtx))
	}

	// 客服系统路由（Web客服端使用，使用 CS 服务自己的 session 认证，不走 FZM-SIGNATURE）
	csRoute := root.Group("/cs")
	{
		csRoute.POST("/login", cs.LoginHandler(serverCtx))
		csRoute.POST("/agents", cs.GetAgentsHandler(serverCtx))
		csRoute.POST("/agent/info", cs.GetAgentInfoHandler(serverCtx))
		csRoute.POST("/agent/status", cs.UpdateAgentStatusHandler(serverCtx))
		csRoute.POST("/token/refresh", cs.RefreshFZMTokenHandler(serverCtx))
		csRoute.POST("/chain/ping", cs.ChainPingHandler(serverCtx))
		csRoute.POST("/chain/bind-comet", cs.BindCometHandler(serverCtx))
		csRoute.POST("/encrypt", cs.EncryptMsgHandler(serverCtx))
		csRoute.POST("/decrypt", cs.DecryptMsgHandler(serverCtx))
		csRoute.POST("/user-pubkey", cs.GetUserPubKeyHandler(serverCtx))
		csRoute.POST("/register-pubkey", cs.RegisterPubKeyHandler(serverCtx))
		csRoute.POST("/verify-chain-pubkey", cs.VerifyChainPubKeyHandler(serverCtx))
	}

	return e
}

func SetupGroupRoutes(e *gin.Engine) *gin.Engine {
	logMiddleware := logger.NewMiddleware(xlog.New(serverCtx.Config().Env, "group"))

	groupApp := e.Group("/group/app")
	groupApp.Use(api.RespMiddleWare(), api.AuthMiddleWare(), trace.TraceMiddleware(), logMiddleware.Handle())
	{
		groupApp.POST("/create-group", group.CreateGroupHandler(serverCtx))
		groupApp.POST("/invite-group-members", group.InviteGroupMembersHandler(serverCtx))
		groupApp.POST("/group-exit", group.GroupExitHandler(serverCtx))
		groupApp.POST("/group-disband", group.GroupDisbandHandler(serverCtx))
		groupApp.POST("/group-remove", group.GroupRemoveHandler(serverCtx))
		groupApp.POST("/change-owner", group.ChangeOwnerHandler(serverCtx))
		groupApp.POST("/join-group", group.JoinGroupHandler(serverCtx))

		groupApp.POST("/name", group.UpdateGroupNameHandler(serverCtx))
		groupApp.POST("/avatar", group.UpdateGroupAvatarHandler(serverCtx))
		groupApp.POST("/joinType", group.UpdateGroupJoinTypeHandler(serverCtx))
		groupApp.POST("/friendType", group.UpdateGroupFriendTypeHandler(serverCtx))
		groupApp.POST("/muteType", group.UpdateGroupMuteTypeHandler(serverCtx))
		groupApp.POST("/member/name", group.UpdateGroupMemberNameHandler(serverCtx))
		groupApp.POST("/member/type", group.SetAdminHandler(serverCtx))
		groupApp.POST("/member/muteTime", group.UpdateGroupMemberMuteTimeHandler(serverCtx))

		groupApp.POST("/group-info", group.GetGroupInfoHandler(serverCtx))
		groupApp.POST("/group-pub-info", group.GetGroupPubInfoHandler(serverCtx))
		groupApp.POST("/group-list", group.GetGroupListHandler(serverCtx))
		groupApp.POST("/group-member-list", group.GetGroupMemberListHandler(serverCtx))
		groupApp.POST("/group-member-info", group.GetGroupMemberInfoHandler(serverCtx))
		groupApp.POST("/mute-list", group.GetMuteListHandler(serverCtx))

	}
	return e
}

func SetupResourceRoutes(e *gin.Engine) *gin.Engine {
	// swagger 文档接口
	if serverCtx.Config().Env == "debug" {
		// todo : 单独开一个 swagger 服务
		e.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}
	return e
}
