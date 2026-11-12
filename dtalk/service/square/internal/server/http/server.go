package http

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/web3chat/txchat/dtalk/service/square/internal/config"
	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
	adminh "github.com/web3chat/txchat/dtalk/service/square/internal/handler/http/admin"
	apph "github.com/web3chat/txchat/dtalk/service/square/internal/handler/http/app"
	"github.com/web3chat/txchat/dtalk/service/square/internal/middleware"
	"github.com/web3chat/txchat/dtalk/service/square/internal/ossclient"
	"github.com/web3chat/txchat/dtalk/service/square/internal/usecase"
)

func New(svc *usecase.Service) *http.Server {
	cfg := svc.Config()
	if config.IsProd(cfg.Env) {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	// 登录静默按 TCP 对端计。不信 X-Forwarded-For，避免客户端伪造 IP 绕过。
	if err := r.SetTrustedProxies(nil); err != nil {
		slog.Error("disable forwarded client ip failed", "err", err)
	}
	r.MaxMultipartMemory = ossclient.MaxBytes
	r.Use(gin.Recovery(), gin.Logger())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORS,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "FZM-SIGNATURE"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/square/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	appH := apph.New(svc)
	adminH := adminh.New(svc)

	appG := r.Group("/square/app", middleware.AppAuth(cfg))
	{
		appG.GET("/status", appH.SquareStatus)
		appG.POST("/posts", appH.CreatePost)
		appG.GET("/posts", appH.ListPosts)
		appG.GET("/posts/:id", appH.GetPost)
		appG.PUT("/posts/:id", appH.UpdatePost)
		appG.DELETE("/posts/:id", appH.DeletePost)
		appG.POST("/posts/:id/like", appH.Like)
		appG.DELETE("/posts/:id/like", appH.Unlike)
		appG.POST("/posts/:id/comments", appH.CreateComment)
		appG.GET("/posts/:id/comments", appH.ListComments)
		appG.GET("/posts/:id/comments/:commentId/replies", appH.ListReplies)
		appG.GET("/replies", appH.ListReplyInbox)
		appG.POST("/replies/read", appH.MarkReplyInboxRead)
		appG.PUT("/posts/:id/comments-enabled", appH.EnableComments)
		appG.DELETE("/comments/:id", appH.DeleteComment)
		appG.GET("/officials", appH.ListOfficials)
		appG.GET("/official/me", appH.OfficialMe)
		appG.POST("/official/apply", appH.ApplyOfficial)
		appG.PUT("/official", appH.UpdateOfficial)
		appG.POST("/follows", appH.Follow)
		appG.DELETE("/follows/:addr", appH.Unfollow)
		appG.GET("/follows", appH.ListFollows)
		appG.GET("/followers", appH.ListFollowers)
		appG.GET("/me/posts", appH.ListMyPosts)
		appG.POST("/posts/:id/report", appH.ReportPost)
	}

	adminPub := r.Group("/square/admin")
	{
		adminPub.GET("/auth/captcha", adminH.Captcha)
		adminPub.POST("/auth/login", adminH.Login)
	}
	adminAuth := r.Group("/square/admin", middleware.AdminJWT(svc))
	{
		adminAuth.GET("/me", adminH.Me)
		adminAuth.POST("/logout", adminH.Logout)
		adminAuth.PUT("/password", adminH.ChangePassword)
		adminAuth.GET("/stats", adminH.Overview)
		adminAuth.GET("/applies", adminH.ListApply)
		adminAuth.POST("/applies/:id/review", adminH.ReviewApply)
		adminAuth.GET("/officials", adminH.ListOfficial)
		adminAuth.POST("/officials/:addr/ban", middleware.RequirePerm(entity.PermOfficialBan), adminH.BanOfficial)
		adminAuth.POST("/officials/:addr/unban", middleware.RequirePerm(entity.PermOfficialBan), adminH.UnbanOfficial)
		adminAuth.POST("/officials/:addr/revoke", middleware.RequirePerm(entity.PermOfficialBan), adminH.RevokeOfficial)
		adminAuth.POST("/officials/:addr/gold", middleware.RequirePerm(entity.PermGold), adminH.GoldOfficial)
		adminAuth.POST("/officials/:addr/ungold", middleware.RequirePerm(entity.PermGold), adminH.UngoldOfficial)
		adminAuth.GET("/platform-official", adminH.PlatformOfficial)
		adminAuth.PUT("/platform-official", adminH.UpdatePlatformOfficial)
		adminAuth.POST("/platform-official/names", adminH.CreateOfficialName)
		adminAuth.PUT("/platform-official/names/:id", adminH.UpdateOfficialName)
		adminAuth.DELETE("/platform-official/names/:id", adminH.DeleteOfficialName)
		adminAuth.POST("/upload", adminH.Upload)
		adminAuth.POST("/posts/official", adminH.CreateOfficialPost)
		adminAuth.PUT("/posts/official/:id", adminH.UpdateOfficialPost)
		adminAuth.GET("/bans", adminH.ListUserBans)
		adminAuth.POST("/bans", adminH.BanUser)
		adminAuth.POST("/bans/:addr/unban", adminH.UnbanUser)
		adminAuth.POST("/bans/:addr", adminH.BanUser)
		adminAuth.GET("/official-replies", adminH.ListOfficialPostReplies)
		adminAuth.GET("/posts", adminH.ListPosts)
		adminAuth.GET("/posts/:id", adminH.GetPost)
		adminAuth.POST("/posts/batch-delete", adminH.BatchDeletePosts)
		adminAuth.POST("/posts/:id/status", adminH.SetPostStatus)
		adminAuth.POST("/posts/:id/pin", adminH.PinPost)
		adminAuth.POST("/posts/:id/unpin", adminH.UnpinPost)
		adminAuth.POST("/posts/:id/ai", adminH.OverridePostAI)
		adminAuth.POST("/posts/:id/comments", adminH.CreateOfficialComment)
		adminAuth.GET("/comments", adminH.ListComments)
		adminAuth.POST("/comments/:id/status", adminH.SetCommentStatus)
		adminAuth.POST("/comments/:id/ai", adminH.OverrideCommentAI)
		adminAuth.GET("/login-alert", adminH.LoginAlert)
		adminAuth.GET("/audits", adminH.ListAudit)
		adminAuth.GET("/accounts", adminH.ListAccounts)
		adminAuth.GET("/authors", adminH.ListAuthorRanks)
		adminAuth.GET("/reports", adminH.ListReports)
		adminAuth.POST("/reports/:id/handle", adminH.HandleReport)
		adminAuth.GET("/settings", adminH.GetSettings)
		adminAuth.PUT("/settings", middleware.RequirePerm(entity.PermSettings), adminH.UpdateSettings)
		adminAuth.POST("/settings/posters", middleware.RequirePerm(entity.PermSettings), adminH.AddPosterAllow)
		adminAuth.DELETE("/settings/posters/:addr", middleware.RequirePerm(entity.PermSettings), adminH.RemovePosterAllow)
		adminAuth.GET("/staff", middleware.RequirePerm(entity.PermStaff), adminH.ListStaff)
		adminAuth.POST("/staff/:id/status", middleware.RequirePerm(entity.PermStaff), adminH.SetStaffStatus)
		adminAuth.PUT("/staff/:id/permissions", middleware.RequirePerm(entity.PermStaff), adminH.SetStaffPerms)
	}

	if cfg.AdminWeb != "" {
		if st, err := os.Stat(cfg.AdminWeb); err == nil && st.IsDir() {
			const consolePath = "/square/console"
			r.Static(consolePath, cfg.AdminWeb)
			r.GET("/square/admin-ui", func(c *gin.Context) {
				c.Redirect(http.StatusMovedPermanently, consolePath+"/")
			})
			r.GET("/square/admin-ui/*filepath", func(c *gin.Context) {
				c.Redirect(http.StatusMovedPermanently, consolePath+c.Param("filepath"))
			})
			slog.Info("square console mounted", "url", consolePath+"/", "dir", cfg.AdminWeb)
		} else {
			slog.Warn("square console dir missing, skip static", "path", cfg.AdminWeb, "err", err)
		}
	}

	return &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}
}
