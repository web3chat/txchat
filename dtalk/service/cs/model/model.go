package model

import "time"

const (
	AgentStatusOffline = 0
	AgentStatusOnline  = 1
	AgentStatusBusy    = 2
)

type CSAgent struct {
	ID            int64     `json:"id"`
	AgentID       string    `json:"agentId"`
	Username      string    `json:"username"`
	PasswordHash  string    `json:"-"`
	PrivateKeyEnc string    `json:"-"`
	PublicKey     string    `json:"publicKey"`
	Name          string    `json:"name"`
	Avatar        string    `json:"avatar"`
	Status        int8      `json:"status"`
	SortOrder     int       `json:"sortOrder"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type CSAgentPublic struct {
	AgentID string `json:"agentId"`
	Name    string `json:"name"`
	Avatar  string `json:"avatar"`
	Status  int8   `json:"status"`
}

// -------- 请求/响应 --------

type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResp struct {
	SessionToken string        `json:"sessionToken"`
	Agent        *CSAgentPublic `json:"agent"`
	// CometReceiveAckDelayMillis 收到 ReceiveMsg 后延迟再发 Ack（毫秒），与 cs.toml CometReceiveAckDelayMillis 一致
	CometReceiveAckDelayMillis int `json:"cometReceiveAckDelayMillis"`
}

type RefreshTokenResp struct {
	FZMSignature string `json:"fzmSignature"`
}

type AgentStatusReq struct {
	Status int8 `json:"status"`
}

type AgentListResp struct {
	List []*CSAgentPublic `json:"list"`
}

type AgentInfoResp struct {
	Agent *CSAgentPublic `json:"agent"`
	CometReceiveAckDelayMillis int `json:"cometReceiveAckDelayMillis"`
}

// -------- 链上绑定 Comet（与 ChatPro 服务器分组一致）--------

type ChainPingReq struct {
	ChainRPCURL string `json:"chainRpcUrl" binding:"required"`
}

type ChainPingResp struct {
	Height int64 `json:"height"`
}

type BindCometReq struct {
	ChainRPCURL string `json:"chainRpcUrl"` // 可空，空则用 cs.toml [Chain] RPC
	CometURL    string `json:"cometUrl" binding:"required"`
}

type BindCometResp struct {
	TxHash string `json:"txHash"`
}

// -------- 消息加密/解密 --------

type EncryptMsgReq struct {
	TargetAddress string `json:"targetAddress" binding:"required"`
	Data          string `json:"data" binding:"required"` // base64-encoded plaintext
}

type EncryptMsgResp struct {
	Data string `json:"data"` // base64-encoded ciphertext
}

type DecryptMsgReq struct {
	SenderAddress string `json:"senderAddress" binding:"required"`
	Data          string `json:"data" binding:"required"` // base64-encoded ciphertext
}

type DecryptMsgResp struct {
	Data string `json:"data"` // base64-encoded plaintext
}

type UserPubKeyReq struct {
	TargetAddress string `json:"targetAddress" binding:"required"`
}

type UserPubKeyResp struct {
	PublicKey string `json:"publicKey"` // hex-encoded compressed public key
}
