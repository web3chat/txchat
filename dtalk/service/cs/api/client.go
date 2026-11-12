package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/web3chat/txchat/dtalk/service/cs/model"
)

// Client 是 CS 微服务的 HTTP 客户端，供 Gateway 调用
type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

type apiResponse struct {
	Result  int             `json:"result"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (c *Client) post(path string, sessionToken string, reqBody interface{}, result interface{}) error {
	var bodyReader io.Reader
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest("POST", c.baseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if sessionToken != "" {
		req.Header.Set("Authorization", "Bearer "+sessionToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return fmt.Errorf("unmarshal response: %w, body: %s", err, string(respBody))
	}
	if apiResp.Result != 0 {
		return fmt.Errorf("cs service error: %s (code=%d)", apiResp.Message, apiResp.Result)
	}

	if result != nil && len(apiResp.Data) > 0 {
		return json.Unmarshal(apiResp.Data, result)
	}
	return nil
}

// Login 客服登录
func (c *Client) Login(req *model.LoginReq) (*model.LoginResp, error) {
	var resp model.LoginResp
	if err := c.post("/cs/login", "", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// RefreshFZMToken 刷新 FZM-SIGNATURE（需 session token）
func (c *Client) RefreshFZMToken(sessionToken string) (*model.RefreshTokenResp, error) {
	var resp model.RefreshTokenResp
	if err := c.post("/cs/token/refresh", sessionToken, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetAgentList 获取客服列表
func (c *Client) GetAgentList() (*model.AgentListResp, error) {
	var resp model.AgentListResp
	if err := c.post("/cs/agents", "", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetAgentInfo 查询客服身份（需 session token）
func (c *Client) GetAgentInfo(sessionToken string) (*model.AgentInfoResp, error) {
	var resp model.AgentInfoResp
	if err := c.post("/cs/agent/info", sessionToken, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateAgentStatus 更新客服状态（需 session token）
func (c *Client) UpdateAgentStatus(sessionToken string, status int8) error {
	return c.post("/cs/agent/status", sessionToken, &model.AgentStatusReq{Status: status}, nil)
}

// ChainPing 探测区块链节点高度
func (c *Client) ChainPing(sessionToken string, req *model.ChainPingReq) (*model.ChainPingResp, error) {
	var resp model.ChainPingResp
	if err := c.post("/cs/chain/ping", sessionToken, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// BindComet 将 Comet 地址绑定到链上账号
func (c *Client) BindComet(sessionToken string, req *model.BindCometReq) (*model.BindCometResp, error) {
	var resp model.BindCometResp
	if err := c.post("/cs/chain/bind-comet", sessionToken, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// EncryptMsg 加密消息体
func (c *Client) EncryptMsg(sessionToken string, req *model.EncryptMsgReq) (*model.EncryptMsgResp, error) {
	var resp model.EncryptMsgResp
	if err := c.post("/cs/encrypt", sessionToken, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DecryptMsg 解密消息体
func (c *Client) DecryptMsg(sessionToken string, req *model.DecryptMsgReq) (*model.DecryptMsgResp, error) {
	var resp model.DecryptMsgResp
	if err := c.post("/cs/decrypt", sessionToken, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetUserPubKey 查询用户链上公钥
func (c *Client) GetUserPubKey(sessionToken string, req *model.UserPubKeyReq) (*model.UserPubKeyResp, error) {
	var resp model.UserPubKeyResp
	if err := c.post("/cs/user-pubkey", sessionToken, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// RegisterPubKeyOnChain 将 agent 公钥注册到链上
func (c *Client) RegisterPubKeyOnChain(sessionToken string) (map[string]string, error) {
	var resp map[string]string
	if err := c.post("/cs/register-pubkey", sessionToken, struct{}{}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// VerifyChainPubKey 诊断：检查 agent 公钥是否在链上
func (c *Client) VerifyChainPubKey(sessionToken string) (map[string]interface{}, error) {
	var resp map[string]interface{}
	if err := c.post("/cs/verify-chain-pubkey", sessionToken, struct{}{}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
