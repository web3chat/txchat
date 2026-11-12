package service

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/web3chat/txchat/dtalk/service/cs/chain"
)

// withChainHint 在常见链错误后附加运维说明（不改变原始错误文本，便于检索）。
func withChainHint(err error) error {
	if err == nil {
		return nil
	}
	s := err.Error()
	if strings.Contains(s, "ErrNoBalance") {
		return fmt.Errorf("%s\n【说明】ErrNoBalance：代扣地址（[Chain] NoBalancePrivateKey 对应链上地址）主币余额不足，或代扣配置与节点不匹配。请向该代扣地址充值主币，并确认私钥与 ChatPro 的 NO_BALANCE_PRIVATE_KEY 一致", s)
	}
	return err
}

// PingChainRPC 探测区块链 JSON-RPC 是否可用（与 App 中合约节点一致，一般为 :8901）。
func (s *Service) PingChainRPC(chainRPC string) (int64, error) {
	return chain.GetBlockHeight(strings.TrimSpace(chainRPC))
}

// BindCometServerOnChain 将 Comet 的 HTTP 根地址写入当前客服链上账号的「服务器分组」，流程对齐 ChatPro-Android：
// chat.CreateRawUpdateServerGroupTx → Chain33.CreateNoBalanceTransaction（代扣）→ 本地库所存私钥签名 → Chain33.SendTransaction。
//
// chainRPC 可传空，此时使用 cs.toml 的 [Chain] RPC；否则使用本次请求指定的节点（与 Web 登录页一致）。
// 需在 cs.toml 中配置 NoBalancePrivateKey（与 App 的 NO_BALANCE_PRIVATE_KEY 一致），否则无法完成代扣与上链。
func (s *Service) BindCometServerOnChain(agentID string, cometHTTPURL string, chainRPC string) (txHash string, err error) {
	cfg := s.Config()
	rpc := strings.TrimSpace(chainRPC)
	if rpc == "" && cfg.Chain != nil {
		rpc = strings.TrimSpace(cfg.Chain.RPC)
	}
	if rpc == "" {
		return "", fmt.Errorf("未指定区块链节点：请在请求中传 chainRpcUrl 或在 cs.toml 中配置 [Chain] RPC（如 http://localhost:8901）")
	}
	if cfg.Chain == nil {
		return "", fmt.Errorf("未配置 [Chain] 段（cs.toml）")
	}
	nb := strings.TrimSpace(cfg.Chain.NoBalancePrivateKey)
	if nb == "" {
		return "", fmt.Errorf("未配置 [Chain] NoBalancePrivateKey，无法创建代扣交易；请配置与 ChatPro 相同的代扣私钥，或使用手机 App「服务器」完成绑定")
	}

	agent, err := s.dao.GetAgentByID(agentID)
	if err != nil {
		return "", err
	}
	if agent == nil {
		return "", fmt.Errorf("agent not found")
	}
	priv, err := s.decryptPrivateKey(agent.PrivateKeyEnc)
	if err != nil {
		return "", fmt.Errorf("decode agent private key: %w", err)
	}
	agentPrivHex := hex.EncodeToString(priv)

	value := normalizeCometHTTPURL(cometHTTPURL)
	if value == "" {
		return "", fmt.Errorf("empty comet url")
	}

	// 1) 创建原始合约交易（与 NetUserDataSource.modifyServerGroup 一致）
	type groupItem struct {
		ID    string `json:"id"`
		Type  int    `json:"type"` // 1=添加
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	rawParam := map[string]interface{}{
		"groups": []groupItem{
			{ID: "", Type: 1, Name: "default", Value: value},
		},
	}
	rawRes, err := chain.Call(rpc, "chat.CreateRawUpdateServerGroupTx", rawParam)
	if err != nil {
		return "", withChainHint(fmt.Errorf("CreateRawUpdateServerGroupTx: %w", err))
	}
	var rawHex string
	if err := json.Unmarshal(rawRes, &rawHex); err != nil {
		return "", fmt.Errorf("parse raw tx hex: %w raw=%s", err, string(rawRes))
	}

	// 2) 代扣包装（TransactionParams.createNoBalanceTx）
	nbParam := map[string]interface{}{
		"addr":      nil,
		"privkey":   nb,
		"txHex":     rawHex,
		"expire":    "1h",
		"index":     0,
		"token":     nil,
		"fee":       int64(0),
		"newToAddr": nil,
	}
	nbRes, err := chain.Call(rpc, "Chain33.CreateNoBalanceTransaction", nbParam)
	if err != nil {
		return "", withChainHint(fmt.Errorf("CreateNoBalanceTransaction: %w", err))
	}
	var nbHex string
	if err := json.Unmarshal(nbRes, &nbHex); err != nil {
		return "", fmt.Errorf("parse no-balance tx hex: %w raw=%s", err, string(nbRes))
	}

	// 3) 使用客服私钥签名（对齐 TransactionSource.signRawTx + 费率；此处费率取配置或默认）
	fee := cfg.Chain.SignFee
	if fee <= 0 {
		fee = 200000
	}
	signParam := map[string]interface{}{
		"addr":      nil,
		"privkey":   agentPrivHex,
		"txHex":     nbHex,
		"expire":    "1h",
		"index":     2,
		"token":     nil,
		"fee":       fee,
		"newToAddr": nil,
	}
	signRes, err := chain.Call(rpc, "Chain33.SignRawTx", signParam)
	if err != nil {
		return "", withChainHint(fmt.Errorf("SignRawTx: %w", err))
	}
	var signedHex string
	if err := json.Unmarshal(signRes, &signedHex); err != nil {
		return "", fmt.Errorf("parse signed tx hex: %w raw=%s", err, string(signRes))
	}

	// 4) 广播
	sendParam := map[string]interface{}{"data": signedHex}
	sendRes, err := chain.Call(rpc, "Chain33.SendTransaction", sendParam)
	if err != nil {
		return "", withChainHint(fmt.Errorf("SendTransaction: %w", err))
	}
	var hash string
	if err := json.Unmarshal(sendRes, &hash); err != nil {
		// 有的节点返回对象
		var obj map[string]interface{}
		if err2 := json.Unmarshal(sendRes, &obj); err2 == nil {
			if h, ok := obj["hash"].(string); ok {
				return h, nil
			}
		}
		return "", fmt.Errorf("parse tx hash: %w raw=%s", err, string(sendRes))
	}
	return hash, nil
}

// RegisterAgentPubKeyOnChain 将 CS agent 的资料写入链上 chat 合约用户字段（与 Android Field 一致），
// 含 pubKey；若 DB 中 name、avatar 非空则一并写入 nickname、avatar，供谈信等客户端展示会话名称与头像。
// 流程对齐 Android 的 NetUserDataSource.setUserInfo：
// chat.CreateRawUpdateUserTx → Chain33.CreateNoBalanceTransaction → SignRawTx → SendTransaction
func (s *Service) RegisterAgentPubKeyOnChain(agentID string) (txHash string, err error) {
	cfg := s.Config()
	rpc := ""
	if cfg.Chain != nil {
		rpc = strings.TrimSpace(cfg.Chain.RPC)
	}
	if rpc == "" {
		return "", fmt.Errorf("未配置 [Chain] RPC")
	}
	if cfg.Chain == nil {
		return "", fmt.Errorf("未配置 [Chain] 段")
	}
	nb := strings.TrimSpace(cfg.Chain.NoBalancePrivateKey)
	if nb == "" {
		return "", fmt.Errorf("未配置 [Chain] NoBalancePrivateKey")
	}

	agent, err := s.dao.GetAgentByID(agentID)
	if err != nil {
		return "", err
	}
	if agent == nil {
		return "", fmt.Errorf("agent not found")
	}
	if agent.PublicKey == "" {
		return "", fmt.Errorf("agent public key is empty in database")
	}
	priv, err := s.decryptPrivateKey(agent.PrivateKeyEnc)
	if err != nil {
		return "", fmt.Errorf("decode agent private key: %w", err)
	}
	agentPrivHex := hex.EncodeToString(priv)

	s.log.Info("RegisterAgentPubKeyOnChain 开始",
		"agentID", agentID,
		"pubKey", agent.PublicKey,
		"name", strings.TrimSpace(agent.Name),
		"avatar_len", len(strings.TrimSpace(agent.Avatar)),
		"rpc", rpc,
	)

	type field struct {
		Name  string `json:"name"`
		Value string `json:"value"`
		Type  int    `json:"type"`
		Level string `json:"level"`
	}

	fields := []field{
		{Name: "pubKey", Value: agent.PublicKey, Type: 1, Level: "public"},
	}
	if n := strings.TrimSpace(agent.Name); n != "" {
		fields = append(fields, field{Name: "nickname", Value: n, Type: 1, Level: "public"})
	}
	if av := strings.TrimSpace(agent.Avatar); av != "" {
		fields = append(fields, field{Name: "avatar", Value: av, Type: 1, Level: "public"})
	}

	// 1) 创建原始合约交易（对齐 Android Field(name, value, type, level)）
	rawParam := map[string]interface{}{
		"fields": fields,
	}
	rawRes, err := chain.Call(rpc, "chat.CreateRawUpdateUserTx", rawParam)
	if err != nil {
		return "", withChainHint(fmt.Errorf("CreateRawUpdateUserTx: %w", err))
	}
	var rawHex string
	if err := json.Unmarshal(rawRes, &rawHex); err != nil {
		return "", fmt.Errorf("parse raw tx hex: %w raw=%s", err, string(rawRes))
	}

	// 2) 代扣包装
	nbParam := map[string]interface{}{
		"addr":      nil,
		"privkey":   nb,
		"txHex":     rawHex,
		"expire":    "1h",
		"index":     0,
		"token":     nil,
		"fee":       int64(0),
		"newToAddr": nil,
	}
	nbRes, err := chain.Call(rpc, "Chain33.CreateNoBalanceTransaction", nbParam)
	if err != nil {
		return "", withChainHint(fmt.Errorf("CreateNoBalanceTransaction: %w", err))
	}
	var nbHex string
	if err := json.Unmarshal(nbRes, &nbHex); err != nil {
		return "", fmt.Errorf("parse no-balance tx hex: %w raw=%s", err, string(nbRes))
	}

	// 3) 使用客服私钥签名
	fee := cfg.Chain.SignFee
	if fee <= 0 {
		fee = 200000
	}
	signParam := map[string]interface{}{
		"addr":      nil,
		"privkey":   agentPrivHex,
		"txHex":     nbHex,
		"expire":    "1h",
		"index":     2,
		"token":     nil,
		"fee":       fee,
		"newToAddr": nil,
	}
	signRes, err := chain.Call(rpc, "Chain33.SignRawTx", signParam)
	if err != nil {
		return "", withChainHint(fmt.Errorf("SignRawTx: %w", err))
	}
	var signedHex string
	if err := json.Unmarshal(signRes, &signedHex); err != nil {
		return "", fmt.Errorf("parse signed tx hex: %w raw=%s", err, string(signRes))
	}

	// 4) 广播
	sendParam := map[string]interface{}{"data": signedHex}
	sendRes, err := chain.Call(rpc, "Chain33.SendTransaction", sendParam)
	if err != nil {
		return "", withChainHint(fmt.Errorf("SendTransaction: %w", err))
	}
	var hash string
	if err := json.Unmarshal(sendRes, &hash); err != nil {
		var obj map[string]interface{}
		if err2 := json.Unmarshal(sendRes, &obj); err2 == nil {
			if h, ok := obj["hash"].(string); ok {
				s.log.Info("RegisterAgentPubKeyOnChain 成功", "agentID", agentID, "txHash", h)
				return h, nil
			}
		}
		return "", fmt.Errorf("parse tx hash: %w raw=%s", err, string(sendRes))
	}
	s.log.Info("RegisterAgentPubKeyOnChain 成功", "agentID", agentID, "txHash", hash)
	return hash, nil
}

func normalizeCometHTTPURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	// Comet WebSocket 形如 ws://host:port → 链上一般存 http(s) 根，与 Android ServerNode 一致
	u = strings.TrimSuffix(u, "/")
	u = strings.TrimSuffix(u, "/sub")
	if strings.HasPrefix(u, "ws://") {
		return "http://" + strings.TrimPrefix(u, "ws://")
	}
	if strings.HasPrefix(u, "wss://") {
		return "https://" + strings.TrimPrefix(u, "wss://")
	}
	return u
}
