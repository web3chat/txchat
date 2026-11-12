package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/web3chat/txchat/dtalk/service/square/internal/config"
	"github.com/web3chat/txchat/dtalk/service/square/internal/usecase"
	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
	"github.com/web3chat/txchat/dtalk/service/square/pkg/auth"
)

const (
	CtxAddress = "address"
	CtxAdmin   = "admin"
)

const (
	mockAddr  = "1FKxgaEh5fuSm7a35BfUnKYAmradowpiTR"
	mockAddr2 = "1AsPsahP7FvpR7F2de1LhSB4SU5ShqZ7eu"
)

func AppAuth(cfg *config.Config) gin.HandlerFunc {
	allowMock := config.IsMockAllowed(cfg.Env, cfg.AllowMock)
	return func(c *gin.Context) {
		sig := strings.TrimSpace(c.GetHeader("FZM-SIGNATURE"))
		if sig == "MOCK" || sig == "MOCK2" {
			if !allowMock {
				slog.Warn("mock signature rejected", "env", cfg.Env)
				abortApp(c, xerror.New(xerror.SignatureInvalid, "无效签名"))
				return
			}
			addr := mockAddr
			if sig == "MOCK2" {
				addr = mockAddr2
			}
			c.Set(CtxAddress, addr)
			c.Next()
			return
		}
		server := auth.NewDefaultApiAuthenticator()
		uid, err := server.Auth(sig)
		if err != nil {
			if err == auth.ERR_SIGNATUREEXPIRED {
				abortApp(c, xerror.New(xerror.SignatureExpired, "签名过期"))
				return
			}
			slog.Debug("verify signature failed", "err", err)
			abortApp(c, xerror.New(xerror.SignatureInvalid, "无效签名"))
			return
		}
		c.Set(CtxAddress, uid)
		c.Next()
	}
}

func AdminJWT(svc *usecase.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		token := strings.TrimPrefix(h, "Bearer ")
		token = strings.TrimSpace(token)
		claims, err := svc.AuthorizeAdmin(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{"code": 401, "message": xerror.MessageOf(err), "data": nil})
			return
		}
		c.Set(CtxAdmin, claims)
		c.Next()
	}
}

func RequirePerm(code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cl := Admin(c)
		if cl == nil {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{"code": 401, "message": "未登录", "data": nil})
			return
		}
		if !cl.Can(code) {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{"code": 403, "message": "权限不足", "data": nil})
			return
		}
		c.Next()
	}
}

func abortApp(c *gin.Context, err error) {
	c.AbortWithStatusJSON(http.StatusOK, gin.H{
		"result":  xerror.CodeOf(err),
		"message": xerror.MessageOf(err),
		"data":    nil,
	})
}

func Address(c *gin.Context) string {
	v, _ := c.Get(CtxAddress)
	s, _ := v.(string)
	return s
}

func Admin(c *gin.Context) *usecase.Claims {
	v, _ := c.Get(CtxAdmin)
	cl, _ := v.(*usecase.Claims)
	return cl
}
