package cs

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/svc"
	"github.com/web3chat/txchat/dtalk/pkg/api"
	xerror "github.com/web3chat/txchat/dtalk/pkg/error"
	"github.com/web3chat/txchat/dtalk/service/cs/model"
)

func getSessionToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	return strings.TrimPrefix(h, "Bearer ")
}

// LoginHandler 客服登录（用户名+密码）— 公开接口
func LoginHandler(ctx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := &model.LoginReq{}
		if err := c.ShouldBindJSON(req); err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.ParamsError).SetExtMessage(err.Error()))
			return
		}

		resp, err := ctx.CSClient.Login(req)
		if err != nil {
			// CS 不可用或网络错误，与「签名」无关，避免误导为无效签名
			c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
			return
		}
		c.Set(api.ReqResult, resp)
		c.Set(api.ReqError, nil)
	}
}

// RefreshFZMTokenHandler 刷新 FZM-SIGNATURE — 需 session 认证
func RefreshFZMTokenHandler(ctx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := getSessionToken(c)
		if token == "" {
			c.Set(api.ReqError, xerror.NewError(xerror.SignatureInvalid).SetExtMessage("missing session token"))
			return
		}

		resp, err := ctx.CSClient.RefreshFZMToken(token)
		if err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
			return
		}
		c.Set(api.ReqResult, resp)
		c.Set(api.ReqError, nil)
	}
}

// GetAgentsHandler 获取客服列表（含在线状态）— 公开接口
func GetAgentsHandler(ctx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		resp, err := ctx.CSClient.GetAgentList()
		if err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
			return
		}
		c.Set(api.ReqResult, resp)
		c.Set(api.ReqError, nil)
	}
}

// GetAgentInfoHandler 查询当前登录客服身份 — 需 session 认证
func GetAgentInfoHandler(ctx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := getSessionToken(c)
		if token == "" {
			c.Set(api.ReqError, xerror.NewError(xerror.SignatureInvalid).SetExtMessage("missing session token"))
			return
		}

		resp, err := ctx.CSClient.GetAgentInfo(token)
		if err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
			return
		}
		c.Set(api.ReqResult, resp)
		c.Set(api.ReqError, nil)
	}
}

// UpdateAgentStatusHandler 客服更新自己的在线状态 — 需 session 认证
func UpdateAgentStatusHandler(ctx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := &model.AgentStatusReq{}
		if err := c.ShouldBindJSON(req); err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.ParamsError).SetExtMessage(err.Error()))
			return
		}

		token := getSessionToken(c)
		if token == "" {
			c.Set(api.ReqError, xerror.NewError(xerror.SignatureInvalid).SetExtMessage("missing session token"))
			return
		}

		if err := ctx.CSClient.UpdateAgentStatus(token, req.Status); err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
			return
		}
		c.Set(api.ReqResult, gin.H{"status": req.Status})
		c.Set(api.ReqError, nil)
	}
}

// ChainPingHandler 探测区块链 JSON-RPC
func ChainPingHandler(ctx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := &model.ChainPingReq{}
		if err := c.ShouldBindJSON(req); err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.ParamsError).SetExtMessage(err.Error()))
			return
		}
		token := getSessionToken(c)
		if token == "" {
			c.Set(api.ReqError, xerror.NewError(xerror.SignatureInvalid).SetExtMessage("missing session token"))
			return
		}
		resp, err := ctx.CSClient.ChainPing(token, req)
		if err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
			return
		}
		c.Set(api.ReqResult, resp)
		c.Set(api.ReqError, nil)
	}
}

// BindCometHandler 链上绑定 Comet
func BindCometHandler(ctx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := &model.BindCometReq{}
		if err := c.ShouldBindJSON(req); err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.ParamsError).SetExtMessage(err.Error()))
			return
		}
		token := getSessionToken(c)
		if token == "" {
			c.Set(api.ReqError, xerror.NewError(xerror.SignatureInvalid).SetExtMessage("missing session token"))
			return
		}
		resp, err := ctx.CSClient.BindComet(token, req)
		if err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
			return
		}
		c.Set(api.ReqResult, resp)
		c.Set(api.ReqError, nil)
	}
}

// EncryptMsgHandler 加密消息体
func EncryptMsgHandler(ctx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := &model.EncryptMsgReq{}
		if err := c.ShouldBindJSON(req); err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.ParamsError).SetExtMessage(err.Error()))
			return
		}
		token := getSessionToken(c)
		if token == "" {
			c.Set(api.ReqError, xerror.NewError(xerror.SignatureInvalid).SetExtMessage("missing session token"))
			return
		}
		resp, err := ctx.CSClient.EncryptMsg(token, req)
		if err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
			return
		}
		c.Set(api.ReqResult, resp)
		c.Set(api.ReqError, nil)
	}
}

// DecryptMsgHandler 解密消息体
func DecryptMsgHandler(ctx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := &model.DecryptMsgReq{}
		if err := c.ShouldBindJSON(req); err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.ParamsError).SetExtMessage(err.Error()))
			return
		}
		token := getSessionToken(c)
		if token == "" {
			c.Set(api.ReqError, xerror.NewError(xerror.SignatureInvalid).SetExtMessage("missing session token"))
			return
		}
		resp, err := ctx.CSClient.DecryptMsg(token, req)
		if err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
			return
		}
		c.Set(api.ReqResult, resp)
		c.Set(api.ReqError, nil)
	}
}

// VerifyChainPubKeyHandler 诊断：检查 agent 公钥是否在链上
func VerifyChainPubKeyHandler(ctx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := getSessionToken(c)
		if token == "" {
			c.Set(api.ReqError, xerror.NewError(xerror.SignatureInvalid).SetExtMessage("missing session token"))
			return
		}
		resp, err := ctx.CSClient.VerifyChainPubKey(token)
		if err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
			return
		}
		c.Set(api.ReqResult, resp)
		c.Set(api.ReqError, nil)
	}
}

// RegisterPubKeyHandler 注册 agent 公钥到链上
func RegisterPubKeyHandler(ctx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := getSessionToken(c)
		if token == "" {
			c.Set(api.ReqError, xerror.NewError(xerror.SignatureInvalid).SetExtMessage("missing session token"))
			return
		}
		resp, err := ctx.CSClient.RegisterPubKeyOnChain(token)
		if err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
			return
		}
		c.Set(api.ReqResult, resp)
		c.Set(api.ReqError, nil)
	}
}

// GetUserPubKeyHandler 查询用户链上公钥
func GetUserPubKeyHandler(ctx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := &model.UserPubKeyReq{}
		if err := c.ShouldBindJSON(req); err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.ParamsError).SetExtMessage(err.Error()))
			return
		}
		token := getSessionToken(c)
		if token == "" {
			c.Set(api.ReqError, xerror.NewError(xerror.SignatureInvalid).SetExtMessage("missing session token"))
			return
		}
		resp, err := ctx.CSClient.GetUserPubKey(token, req)
		if err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
			return
		}
		c.Set(api.ReqResult, resp)
		c.Set(api.ReqError, nil)
	}
}
