package service

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/inconshreveable/log15"
	"github.com/web3chat/txchat/dtalk/pkg/auth"
	xcrypt "github.com/web3chat/txchat/dtalk/pkg/crypt"
	secp256k1 "github.com/web3chat/txchat/dtalk/pkg/crypt/secp256k1-haltingstate"
	"github.com/web3chat/txchat/dtalk/service/cs/chain"
	"github.com/web3chat/txchat/dtalk/service/cs/config"
	cscrypto "github.com/web3chat/txchat/dtalk/service/cs/crypto"
	"github.com/web3chat/txchat/dtalk/service/cs/dao"
	"github.com/web3chat/txchat/dtalk/service/cs/model"
	"golang.org/x/crypto/bcrypt"
)

const sessionTokenTTL = 24 * time.Hour

type CSClaims struct {
	AgentID  string `json:"agentId"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type Service struct {
	log           log15.Logger
	cfg           *config.Config
	dao           *dao.Dao
	authenticator auth.ApiAuthenticator
	crypt         xcrypt.Encrypt
}

func New(cfg *config.Config) *Service {
	driver, err := xcrypt.Load(secp256k1.Name)
	if err != nil {
		panic(err)
	}
	return &Service{
		log:           log15.New("module", "cs/svc"),
		cfg:           cfg,
		dao:           dao.New(cfg),
		authenticator: auth.NewDefaultApiAuthenticator(),
		crypt:         driver,
	}
}

func (s *Service) Config() *config.Config {
	return s.cfg
}

// CometReceiveAckDelayMillis 为 ReceiveMsg 后延迟再发 ReceiveMsgReply 的毫秒数（cs-web 与离线 AI 共用）。
// cs.toml 为 0 或未配置时使用 280，减轻 Pusher 尚未写入 Redis conn-seq 时对端仍显示「未读」。
func (s *Service) CometReceiveAckDelayMillis() int {
	if s.cfg != nil && s.cfg.CometReceiveAckDelayMillis > 0 {
		return s.cfg.CometReceiveAckDelayMillis
	}
	return 280
}

// Login 用户名密码登录，返回 JWT session token
func (s *Service) Login(req *model.LoginReq) (*model.LoginResp, error) {
	s.log.Info("Login 开始", "username", req.Username)

	agent, err := s.dao.GetAgentByUsername(req.Username)
	if err != nil {
		s.log.Error("Login: 查询用户失败", "username", req.Username, "err", err)
		return nil, err
	}
	if agent == nil {
		s.log.Warn("Login: 用户不存在", "username", req.Username)
		return nil, fmt.Errorf("用户名或密码错误")
	}
	s.log.Info("Login: 找到用户",
		"username", req.Username,
		"agentID", agent.AgentID,
		"name", agent.Name,
		"publicKey_len", len(agent.PublicKey),
		"privateKeyEnc_len", len(agent.PrivateKeyEnc),
	)

	if err := bcrypt.CompareHashAndPassword([]byte(agent.PasswordHash), []byte(req.Password)); err != nil {
		s.log.Warn("Login: 密码不匹配", "username", req.Username)
		return nil, fmt.Errorf("用户名或密码错误")
	}
	s.log.Info("Login: 密码验证通过", "username", req.Username)

	now := time.Now()
	claims := &CSClaims{
		AgentID:  agent.AgentID,
		Username: agent.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(sessionTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "cs-service",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		s.log.Error("Login: 生成 JWT 失败", "err", err)
		return nil, fmt.Errorf("生成 session token 失败: %w", err)
	}

	s.log.Info("Login 成功",
		"username", req.Username,
		"agentID", agent.AgentID,
		"jwt_len", len(tokenStr),
	)
	return &model.LoginResp{
		SessionToken: tokenStr,
		Agent: &model.CSAgentPublic{
			AgentID: agent.AgentID,
			Name:    agent.Name,
			Avatar:  agent.Avatar,
			Status:  agent.Status,
		},
		CometReceiveAckDelayMillis: s.CometReceiveAckDelayMillis(),
	}, nil
}

// ValidateSession 校验 JWT session token，返回 agentID
func (s *Service) ValidateSession(tokenStr string) (string, error) {
	claims := &CSClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		s.log.Warn("ValidateSession: session 无效", "err", err)
		return "", fmt.Errorf("session 无效或已过期")
	}
	s.log.Debug("ValidateSession: 通过", "agentID", claims.AgentID, "username", claims.Username)
	return claims.AgentID, nil
}

// GenerateFZMSignature 用服务端存储的私钥为指定客服生成 FZM-SIGNATURE
func (s *Service) GenerateFZMSignature(agentID string) (*model.RefreshTokenResp, error) {
	s.log.Info("GenerateFZMSignature 开始", "agentID", agentID)

	agent, err := s.dao.GetAgentByID(agentID)
	if err != nil {
		s.log.Error("查询 agent 失败", "agentID", agentID, "err", err)
		return nil, err
	}
	if agent == nil {
		s.log.Error("agent not found", "agentID", agentID)
		return nil, fmt.Errorf("agent not found")
	}

	s.log.Info("DB agent 信息",
		"agentID", agent.AgentID,
		"username", agent.Username,
		"publicKey_len", len(agent.PublicKey),
		"privateKeyEnc_len", len(agent.PrivateKeyEnc),
	)

	// 读取私钥：支持 AES-GCM 加密存储和明文存储
	privKey, err := s.decryptPrivateKey(agent.PrivateKeyEnc)
	if err != nil {
		s.log.Error("解密/解码私钥失败", "err", err)
		return nil, fmt.Errorf("decode private key: %w", err)
	}

	pubKey, err := hex.DecodeString(agent.PublicKey)
	if err != nil {
		s.log.Error("解码公钥失败", "err", err)
		return nil, fmt.Errorf("decode public key: %w", err)
	}

	s.log.Info("密钥解码完成", "privKey_len", len(privKey), "pubKey_len", len(pubKey))

	appId := s.cfg.AppId
	s.log.Info("生成 FZM-SIGNATURE", "appId", appId)

	fzmToken := s.authenticator.Request(appId, pubKey, privKey)
	s.log.Info("FZM-SIGNATURE 已生成", "token_len", len(fzmToken), "token_prefix", truncStr(fzmToken, 50))

	// 自验证：用和 Gateway AuthMiddleWare 相同的逻辑验签，确保签名有效
	verifier := auth.NewDefaultApiAuthenticator()
	uid, err := verifier.Auth(fzmToken)
	if err != nil {
		s.log.Error("FZM-SIGNATURE 自验证失败！签名无效，Comet 认证必然会失败",
			"err", err,
			"agentID", agentID,
		)
		return nil, fmt.Errorf("FZM-SIGNATURE 自验证失败: %w (请检查私钥是否正确，是否需要 AES 解密)", err)
	}
	s.log.Info("FZM-SIGNATURE 自验证通过",
		"验签得到地址", uid,
		"期望地址(agentID)", agentID,
		"地址匹配", uid == agentID,
	)
	if uid != agentID {
		s.log.Error("地址不匹配！验签得到的地址与 agentID 不一致，Comet 登录后 Redis 中的 mid 将是错误的",
			"got", uid, "want", agentID)
	}

	return &model.RefreshTokenResp{FZMSignature: fzmToken}, nil
}

// decryptPrivateKey 从 DB 中的 private_key_enc 字段还原原始私钥
// 如果配置了 AESKey 则做 AES-GCM 解密，否则视为明文 hex
func (s *Service) decryptPrivateKey(privateKeyEnc string) ([]byte, error) {
	raw, err := hex.DecodeString(privateKeyEnc)
	if err != nil {
		return nil, fmt.Errorf("hex decode failed: %w", err)
	}

	if s.cfg.AESKey == "" {
		// 没配置 AES 密钥 → private_key_enc 就是明文 hex 编码的私钥
		if len(raw) != 32 {
			return nil, fmt.Errorf("明文私钥长度异常: got %d bytes, want 32 (如果私钥是 AES 加密的，请在 cs.toml 中配置 AESKey)", len(raw))
		}
		s.log.Debug("私钥模式: 明文", "len", len(raw))
		return raw, nil
	}

	// 有 AES 密钥 → 做 AES-GCM 解密
	s.log.Debug("私钥模式: AES-GCM 解密")
	aesKeyBytes, err := hex.DecodeString(s.cfg.AESKey)
	if err != nil || len(aesKeyBytes) != 32 {
		return nil, fmt.Errorf("AESKey 配置无效，需要 64 位 hex 字符串 (32 字节)")
	}

	block, err := aes.NewCipher(aesKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}
	nonceSize := aesGCM.NonceSize()
	if len(raw) < nonceSize {
		return nil, fmt.Errorf("密文太短: %d < nonceSize %d", len(raw), nonceSize)
	}
	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("AES-GCM 解密失败: %w (AESKey 可能不匹配)", err)
	}

	// plaintext 是 privKeyHex 的 []byte（工具加密的是 hex 字符串，不是原始字节）
	privKeyBytes, err := hex.DecodeString(string(plaintext))
	if err != nil {
		return nil, fmt.Errorf("解密后的内容不是有效的 hex 私钥: %w", err)
	}
	if len(privKeyBytes) != 32 {
		return nil, fmt.Errorf("解密后私钥长度异常: got %d bytes, want 32", len(privKeyBytes))
	}
	return privKeyBytes, nil
}

func truncStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// GetAgentList 获取所有客服及其在线状态
func (s *Service) GetAgentList() (*model.AgentListResp, error) {
	s.log.Info("GetAgentList 开始")
	agents, err := s.dao.GetAgentList()
	if err != nil {
		s.log.Error("GetAgentList: 查询失败", "err", err)
		return nil, err
	}

	var list []*model.CSAgentPublic
	for _, a := range agents {
		s.log.Info("GetAgentList: agent", "agentID", a.AgentID, "name", a.Name, "status", a.Status)
		list = append(list, &model.CSAgentPublic{
			AgentID: a.AgentID,
			Name:    a.Name,
			Avatar:  a.Avatar,
			Status:  a.Status,
		})
	}
	if list == nil {
		list = make([]*model.CSAgentPublic, 0)
	}
	s.log.Info("GetAgentList 完成", "count", len(list))
	return &model.AgentListResp{List: list}, nil
}

// GetAgentInfo 查询单个客服信息
func (s *Service) GetAgentInfo(agentID string) (*model.AgentInfoResp, error) {
	s.log.Info("GetAgentInfo 开始", "agentID", agentID)
	agent, err := s.dao.GetAgentByID(agentID)
	if err != nil {
		s.log.Error("GetAgentInfo: 查询失败", "agentID", agentID, "err", err)
		return nil, err
	}
	if agent == nil {
		s.log.Warn("GetAgentInfo: agent 不存在", "agentID", agentID)
		return nil, fmt.Errorf("not a registered agent")
	}
	s.log.Info("GetAgentInfo 成功", "agentID", agentID, "name", agent.Name, "status", agent.Status)
	return &model.AgentInfoResp{
		Agent: &model.CSAgentPublic{
			AgentID: agent.AgentID,
			Name:    agent.Name,
			Avatar:  agent.Avatar,
			Status:  agent.Status,
		},
		CometReceiveAckDelayMillis: s.CometReceiveAckDelayMillis(),
	}, nil
}

// EncryptMsg 使用 agent 私钥和目标用户的公钥，对消息体进行 ECDH+AES-GCM 加密
func (s *Service) EncryptMsg(agentID string, req *model.EncryptMsgReq) (*model.EncryptMsgResp, error) {
	agent, err := s.dao.GetAgentByID(agentID)
	if err != nil || agent == nil {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}
	privKey, err := s.decryptPrivateKey(agent.PrivateKeyEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt agent private key: %w", err)
	}
	privKeyHex := hex.EncodeToString(privKey)

	plaintext, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		return nil, fmt.Errorf("decode plaintext: %w", err)
	}

	targetPubKey, err := s.getUserPubKey(agentID, agent, privKey, req.TargetAddress)
	if err != nil {
		return nil, fmt.Errorf("get user pubkey: %w", err)
	}

	encrypted, err := cscrypto.EncryptWithDHKeyPair(privKeyHex, targetPubKey, plaintext)
	if err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}

	return &model.EncryptMsgResp{Data: base64.StdEncoding.EncodeToString(encrypted)}, nil
}

// DecryptMsg 使用 agent 私钥和发送者的公钥，对消息体进行 ECDH+AES-GCM 解密
func (s *Service) DecryptMsg(agentID string, req *model.DecryptMsgReq) (*model.DecryptMsgResp, error) {
	agent, err := s.dao.GetAgentByID(agentID)
	if err != nil || agent == nil {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}
	privKey, err := s.decryptPrivateKey(agent.PrivateKeyEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt agent private key: %w", err)
	}
	privKeyHex := hex.EncodeToString(privKey)

	ciphertext, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}

	senderPubKey, err := s.getUserPubKey(agentID, agent, privKey, req.SenderAddress)
	if err != nil {
		return nil, fmt.Errorf("get user pubkey: %w", err)
	}

	decrypted, err := cscrypto.DecryptWithDHKeyPair(privKeyHex, senderPubKey, ciphertext)
	if err != nil {
		s.log.Warn("decrypt failed, returning raw data", "err", err, "sender", req.SenderAddress)
		return &model.DecryptMsgResp{Data: req.Data}, nil
	}

	return &model.DecryptMsgResp{Data: base64.StdEncoding.EncodeToString(decrypted)}, nil
}

// GetUserPubKey 查询目标用户的链上公钥
func (s *Service) GetUserPubKeyAPI(agentID, targetAddress string) (*model.UserPubKeyResp, error) {
	agent, err := s.dao.GetAgentByID(agentID)
	if err != nil || agent == nil {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}
	privKey, err := s.decryptPrivateKey(agent.PrivateKeyEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt agent private key: %w", err)
	}

	pubKey, err := s.getUserPubKey(agentID, agent, privKey, targetAddress)
	if err != nil {
		return nil, err
	}
	return &model.UserPubKeyResp{PublicKey: pubKey}, nil
}

func (s *Service) getUserPubKey(agentID string, agent *model.CSAgent, privKey []byte, targetAddress string) (string, error) {
	// 如果目标地址就是当前 agent 自己，直接返回自己的公钥
	if targetAddress == agentID && agent.PublicKey != "" {
		s.log.Info("getUserPubKey: 目标是自己，使用 agent 公钥", "agentID", agentID)
		return agent.PublicKey, nil
	}

	// 查看目标是否也是一个 CS agent（公钥存在 cs_agent 表中）
	targetAgent, err := s.dao.GetAgentByID(targetAddress)
	if err == nil && targetAgent != nil && targetAgent.PublicKey != "" {
		s.log.Info("getUserPubKey: 目标是 CS agent，使用数据库公钥", "target", targetAddress)
		return targetAgent.PublicKey, nil
	}

	// 非 agent 用户，从链上查询公钥
	chainRPC := s.cfg.Chain.RPC
	pubKey, err := chain.GetUserPubKey(chainRPC, agentID, targetAddress, privKey, agent.PublicKey)
	if err != nil {
		return "", fmt.Errorf("chain GetUserPubKey(%s): %w", targetAddress, err)
	}
	return pubKey, nil
}

// VerifyChainPubKey 诊断接口：查询 agent 自己的公钥是否已在链上注册
func (s *Service) VerifyChainPubKey(agentID string) (map[string]interface{}, error) {
	agent, err := s.dao.GetAgentByID(agentID)
	if err != nil || agent == nil {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}
	privKey, err := s.decryptPrivateKey(agent.PrivateKeyEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt agent private key: %w", err)
	}
	result := map[string]interface{}{
		"agentID":    agentID,
		"dbPubKey":   agent.PublicKey,
		"chainRPC":   s.cfg.Chain.RPC,
		"chainQuery": "pending",
	}

	chainPubKey, err := chain.GetUserPubKey(s.cfg.Chain.RPC, agentID, agentID, privKey, agent.PublicKey)
	if err != nil {
		result["chainQuery"] = "failed"
		result["chainError"] = err.Error()
		result["match"] = false
		s.log.Warn("VerifyChainPubKey: 链上查询失败", "agentID", agentID, "err", err)
		return result, nil
	}
	result["chainQuery"] = "ok"
	result["chainPubKey"] = chainPubKey
	result["match"] = (chainPubKey == agent.PublicKey)
	s.log.Info("VerifyChainPubKey", "agentID", agentID, "match", chainPubKey == agent.PublicKey)
	return result, nil
}

// UpdateAgentStatus 更新客服在线状态
func (s *Service) UpdateAgentStatus(agentID string, status int8) error {
	s.log.Info("UpdateAgentStatus 开始", "agentID", agentID, "newStatus", status)
	agent, err := s.dao.GetAgentByID(agentID)
	if err != nil {
		s.log.Error("UpdateAgentStatus: 查询失败", "agentID", agentID, "err", err)
		return err
	}
	if agent == nil {
		s.log.Warn("UpdateAgentStatus: agent 不存在", "agentID", agentID)
		return fmt.Errorf("not a registered agent")
	}
	if err := s.dao.UpdateAgentStatus(agentID, status); err != nil {
		s.log.Error("UpdateAgentStatus: 更新失败", "agentID", agentID, "err", err)
		return err
	}
	s.log.Info("UpdateAgentStatus 成功", "agentID", agentID, "oldStatus", agent.Status, "newStatus", status)
	return nil
}
