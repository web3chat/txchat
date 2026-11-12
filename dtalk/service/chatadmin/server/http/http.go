package http

import (
	"io/fs"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/web3chat/txchat/dtalk/pkg/api"
	"github.com/web3chat/txchat/dtalk/pkg/logger"
	"github.com/web3chat/txchat/dtalk/service/chatadmin/config"
	"github.com/web3chat/txchat/dtalk/service/chatadmin/service"
	manageweb "github.com/web3chat/txchat/dtalk/service/web/manage-web"
)

const srvName = "chatadmin/http"

var (
	svc *service.Service
	log zerolog.Logger
)

func Init(s *service.Service) *http.Server {
	addr := s.Config().HttpServer.Addr
	engine := defaultEngine(s.Config())
	initService(s)
	log = logger.New(s.Config().Env, srvName)
	setupEngine(engine)
	log.Info().Strs("cors_allow_origins", corsAllowOrigins(s.Config())).Msg("http server cors configured")
	pprof.Register(engine)
	srv := &http.Server{
		Addr:    addr,
		Handler: engine,
	}
	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("engineInner.Start()")
			panic(err)
		}
	}()
	return srv
}

func corsAllowOrigins(conf *config.Config) []string {
	if conf != nil && conf.Cors != nil && len(conf.Cors.AllowOrigins) > 0 {
		return conf.Cors.AllowOrigins
	}
	return []string{
		"http://localhost:8000",
		"http://127.0.0.1:8000",
		"http://localhost:8000",
	}
}

func corsAllowCredentials(conf *config.Config) bool {
	if conf != nil && conf.Cors != nil {
		return conf.Cors.AllowCredentials
	}
	return true
}

// defaultEngine returns an Engine instance with the Logger and Recovery middleware already attached.
func defaultEngine(conf *config.Config) *gin.Engine {
	router := gin.New()
	// 拉黑按 TCP 对端计。不信 X-Forwarded-For，避免伪造来源绕过。
	if err := router.SetTrustedProxies(nil); err != nil {
		log.Error().Err(err).Msg("关闭转发 IP 信任失败")
	}
	router.Use(gin.LoggerWithFormatter(api.Chat33GinLogFormatter))
	router.Use(gin.Recovery())

	origins := corsAllowOrigins(conf)
	router.Use(cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: corsAllowCredentials(conf),
		MaxAge:           12 * time.Hour,
	}))

	return router
}

func initService(s *service.Service) {
	svc = s
}

// setupEngine
// @title 群组管理服务接口
// @version 1.0
// @host 127.0.0.1:18020
func setupEngine(e *gin.Engine) *gin.Engine {
	e.Use(securityGuard.middleware())
	e.NoRoute(notFound)

	// 群组管理API
	groupManage := e.Group("/api/v1/groupmanage")
	{
		// 登录鉴权
		auth := groupManage.Group("/auth")
		{
			auth.GET("/nonce", getAuthNonce)
			auth.POST("/verify", verifyAuth)
		}
		authAuthed := groupManage.Group("/auth")
		authAuthed.Use(AuthMiddleware())
		{
			authAuthed.POST("/logout", logoutAuth)
		}

		// 系统配置（需要鉴权）
		config := groupManage.Group("/config")
		config.Use(AuthMiddleware())
		{
			config.GET("", getConfig)
		}

		// 群组信息管理
		groups := groupManage.Group("/groups")
		groups.Use(AuthMiddleware())
		{
			// 具体路径（放在最前面，避免与参数路由冲突）
			groups.GET("/list", getGroupList)
			groups.GET("/stats", getGroupStats)
			groups.GET("/today-count", getTodayGroupCount)
			groups.GET("/by-status/:status", getGroupsByStatus)
			groups.GET("/by-owner", getGroupsByOwner)

			// 使用不同的路径模式避免冲突
			groups.GET("/info/:id", getGroupInfo)
			groups.GET("/:id/members", getGroupMembers)
			groups.PUT("/maximum/:id", updateGroupMaximum)
			groups.PUT("/status/:id", updateGroupStatus)
			groups.PUT("/name/:id", updateGroupName)
		}

		// 用户统计管理
		users := groupManage.Group("/users")
		users.Use(AuthMiddleware())
		{
			users.GET("/total-count", getTotalUserCount)
			users.GET("/today-new-count", getTodayNewUserCount)
			users.GET("/login-logs", getAllLoginLogs)
			users.GET("/login-history", getUserLoginHistory)
		}

		audits := groupManage.Group("/audits")
		audits.Use(AuthMiddleware())
		{
			audits.GET("", listAudits)
		}

		// 白名单管理
		whitelist := groupManage.Group("/whitelist")
		whitelist.Use(AuthMiddleware())
		{
			// 具体路径
			whitelist.GET("/list", getWhitelistList)
			whitelist.GET("/stats", getWhitelistStats)
			whitelist.GET("/by-address", getWhitelistByAddress)
			whitelist.POST("/", addWhitelist)

			// 使用不同的路径模式避免冲突
			whitelist.GET("/info/:id", getWhitelistById)
			whitelist.PUT("/update/:id", updateWhitelist)
			whitelist.DELETE("/delete/:id", deleteWhitelist)
		}

		// 防攻击提醒（需要鉴权，只给已登录运营看）
		security := groupManage.Group("/security")
		security.Use(AuthMiddleware())
		{
			security.GET("/alerts", listSecurityAlerts)
		}

		// 每日活跃指标
		metrics := groupManage.Group("/metrics")
		metrics.Use(AuthMiddleware())
		{
			metrics.GET("/daily-active", listDailyActive)
			metrics.GET("/daily-active/addresses", listActiveAddresses)
			metrics.GET("/daily-active/groups", listActiveGroups)
			metrics.POST("/daily-active/snapshot", snapshotDailyActive)
		}
	}

	mountConsole(e)
	return e
}

// mountConsole 把运营页面挂在 /console/，根路径转到这里。
// 与广场把 web/admin 打进镜像后挂到 /square/console/ 相同，只是 chatadmin 的镜像构建只拷贝二进制，所以页面用 embed 打进程序。
func mountConsole(e *gin.Engine) {
	sub, err := fs.Sub(manageweb.Files, ".")
	if err != nil {
		log.Error().Err(err).Msg("运营页面嵌入失败")
		return
	}
	const consolePath = "/console"
	e.StaticFS(consolePath, http.FS(sub))
	e.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, consolePath+"/")
	})
	log.Info().Str("url", consolePath+"/").Msg("运营页面已挂载")
}
