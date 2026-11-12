package http

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/web3chat/txchat/dtalk/pkg/api"
	xerror "github.com/web3chat/txchat/dtalk/pkg/error"
	"github.com/web3chat/txchat/dtalk/service/cs/model"
)

const agentIDKey = "cs_agent_id"

func SessionAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.GetHeader("Authorization")
		tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
		if tokenStr == "" {
			log.Warn("SessionAuth: missing session token", "path", c.Request.URL.Path)
			c.Set(api.ReqError, xerror.NewError(xerror.SignatureInvalid).SetExtMessage("missing session token"))
			c.Abort()
			return
		}
		agentID, err := svc.ValidateSession(tokenStr)
		if err != nil {
			log.Warn("SessionAuth: session 无效", "path", c.Request.URL.Path, "err", err)
			c.Set(api.ReqError, xerror.NewError(xerror.SignatureExpired).SetExtMessage(err.Error()))
			c.Abort()
			return
		}
		log.Debug("SessionAuth: 认证通过", "path", c.Request.URL.Path, "agentID", agentID)
		c.Set(agentIDKey, agentID)
	}
}

func Login(c *gin.Context) {
	log.Info(">>> POST /cs/login")
	req := &model.LoginReq{}
	if err := c.ShouldBindJSON(req); err != nil {
		log.Error("/cs/login: 参数错误", "err", err)
		c.Set(api.ReqError, xerror.NewError(xerror.ParamsError).SetExtMessage(err.Error()))
		return
	}
	log.Info("/cs/login: 请求参数", "username", req.Username)

	resp, err := svc.Login(req)
	if err != nil {
		log.Error("/cs/login: 登录失败", "username", req.Username, "err", err)
		c.Set(api.ReqError, xerror.NewError(xerror.SignatureInvalid).SetExtMessage(err.Error()))
		return
	}
	log.Info("/cs/login: 登录成功",
		"username", req.Username,
		"agentID", resp.Agent.AgentID,
		"name", resp.Agent.Name,
		"token_len", len(resp.SessionToken),
	)
	c.Set(api.ReqResult, resp)
	c.Set(api.ReqError, nil)
}

func RefreshFZMToken(c *gin.Context) {
	agentID := c.MustGet(agentIDKey).(string)
	log.Info(">>> POST /cs/token/refresh", "agentID", agentID)

	resp, err := svc.GenerateFZMSignature(agentID)
	if err != nil {
		log.Error("/cs/token/refresh: 生成 FZM-SIGNATURE 失败", "agentID", agentID, "err", err)
		c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
		return
	}
	log.Info("/cs/token/refresh: 成功",
		"agentID", agentID,
		"fzmSignature_len", len(resp.FZMSignature),
	)
	c.Set(api.ReqResult, resp)
	c.Set(api.ReqError, nil)
}

func GetAgents(c *gin.Context) {
	log.Info(">>> POST /cs/agents")
	resp, err := svc.GetAgentList()
	if err != nil {
		log.Error("/cs/agents: 失败", "err", err)
		c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
		return
	}
	log.Info("/cs/agents: 成功", "count", len(resp.List))
	c.Set(api.ReqResult, resp)
	c.Set(api.ReqError, nil)
}

func GetAgentInfo(c *gin.Context) {
	agentID := c.MustGet(agentIDKey).(string)
	log.Info(">>> POST /cs/agent/info", "agentID", agentID)
	resp, err := svc.GetAgentInfo(agentID)
	if err != nil {
		log.Error("/cs/agent/info: 失败", "agentID", agentID, "err", err)
		c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
		return
	}
	log.Info("/cs/agent/info: 成功", "agentID", agentID, "name", resp.Agent.Name)
	c.Set(api.ReqResult, resp)
	c.Set(api.ReqError, nil)
}

func AgentUpdateStatus(c *gin.Context) {
	req := &model.AgentStatusReq{}
	if err := c.ShouldBindJSON(req); err != nil {
		log.Error("/cs/agent/status: 参数错误", "err", err)
		c.Set(api.ReqError, xerror.NewError(xerror.ParamsError).SetExtMessage(err.Error()))
		return
	}

	agentID := c.MustGet(agentIDKey).(string)
	log.Info(">>> POST /cs/agent/status", "agentID", agentID, "newStatus", req.Status)
	if err := svc.UpdateAgentStatus(agentID, req.Status); err != nil {
		log.Error("/cs/agent/status: 失败", "agentID", agentID, "err", err)
		c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
		return
	}
	log.Info("/cs/agent/status: 成功", "agentID", agentID, "status", req.Status)
	c.Set(api.ReqResult, gin.H{"status": req.Status})
	c.Set(api.ReqError, nil)
}

// ChainPing 探测区块链 JSON-RPC（如 http://localhost:8901）
func ChainPing(c *gin.Context) {
	req := &model.ChainPingReq{}
	if err := c.ShouldBindJSON(req); err != nil {
		c.Set(api.ReqError, xerror.NewError(xerror.ParamsError).SetExtMessage(err.Error()))
		return
	}
	h, err := svc.PingChainRPC(req.ChainRPCURL)
	if err != nil {
		c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
		return
	}
	c.Set(api.ReqResult, &model.ChainPingResp{Height: h})
	c.Set(api.ReqError, nil)
}

// BindComet 将 Comet 根地址写入当前客服链上账号的 chat 合约服务器分组
func BindComet(c *gin.Context) {
	req := &model.BindCometReq{}
	if err := c.ShouldBindJSON(req); err != nil {
		c.Set(api.ReqError, xerror.NewError(xerror.ParamsError).SetExtMessage(err.Error()))
		return
	}
	agentID := c.MustGet(agentIDKey).(string)
	hash, err := svc.BindCometServerOnChain(agentID, req.CometURL, req.ChainRPCURL)
	if err != nil {
		c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
		return
	}
	c.Set(api.ReqResult, &model.BindCometResp{TxHash: hash})
	c.Set(api.ReqError, nil)
}

// EncryptMsg 加密消息体
func EncryptMsg(c *gin.Context) {
	req := &model.EncryptMsgReq{}
	if err := c.ShouldBindJSON(req); err != nil {
		c.Set(api.ReqError, xerror.NewError(xerror.ParamsError).SetExtMessage(err.Error()))
		return
	}
	agentID := c.MustGet(agentIDKey).(string)
	resp, err := svc.EncryptMsg(agentID, req)
	if err != nil {
		log.Error("/cs/encrypt: 失败", "agentID", agentID, "target", req.TargetAddress, "err", err)
		c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
		return
	}
	c.Set(api.ReqResult, resp)
	c.Set(api.ReqError, nil)
}

// DecryptMsg 解密消息体
func DecryptMsg(c *gin.Context) {
	req := &model.DecryptMsgReq{}
	if err := c.ShouldBindJSON(req); err != nil {
		c.Set(api.ReqError, xerror.NewError(xerror.ParamsError).SetExtMessage(err.Error()))
		return
	}
	agentID := c.MustGet(agentIDKey).(string)
	resp, err := svc.DecryptMsg(agentID, req)
	if err != nil {
		log.Error("/cs/decrypt: 失败", "agentID", agentID, "sender", req.SenderAddress, "err", err)
		c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
		return
	}
	c.Set(api.ReqResult, resp)
	c.Set(api.ReqError, nil)
}

// VerifyChainPubKey 诊断：检查 agent 公钥是否在链上
func VerifyChainPubKey(c *gin.Context) {
	agentID := c.MustGet(agentIDKey).(string)
	result, err := svc.VerifyChainPubKey(agentID)
	if err != nil {
		log.Error("/cs/verify-chain-pubkey: 失败", "agentID", agentID, "err", err)
		c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
		return
	}
	c.Set(api.ReqResult, result)
	c.Set(api.ReqError, nil)
}

// RegisterPubKeyOnChain 将 agent 公钥及 DB 中的显示名/头像同步到链上（nickname、avatar），使谈信等客户端可展示
func RegisterPubKeyOnChain(c *gin.Context) {
	agentID := c.MustGet(agentIDKey).(string)
	txHash, err := svc.RegisterAgentPubKeyOnChain(agentID)
	if err != nil {
		log.Error("/cs/register-pubkey: 失败", "agentID", agentID, "err", err)
		c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
		return
	}
	c.Set(api.ReqResult, map[string]string{"txHash": txHash})
	c.Set(api.ReqError, nil)
}

// GetUserPubKey 查询用户链上公钥
func GetUserPubKey(c *gin.Context) {
	req := &model.UserPubKeyReq{}
	if err := c.ShouldBindJSON(req); err != nil {
		c.Set(api.ReqError, xerror.NewError(xerror.ParamsError).SetExtMessage(err.Error()))
		return
	}
	agentID := c.MustGet(agentIDKey).(string)
	resp, err := svc.GetUserPubKeyAPI(agentID, req.TargetAddress)
	if err != nil {
		log.Error("/cs/user-pubkey: 失败", "agentID", agentID, "target", req.TargetAddress, "err", err)
		c.Set(api.ReqError, xerror.NewError(xerror.CodeInnerError).SetExtMessage(err.Error()))
		return
	}
	c.Set(api.ReqResult, resp)
	c.Set(api.ReqError, nil)
}
