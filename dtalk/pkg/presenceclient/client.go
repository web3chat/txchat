package presenceclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client 调用中心 Presence HTTP API（服务端内部使用）。
// 中心服务已独立为同级仓库 github.com/txchat/presence，本包仅作 HTTP 客户端。
type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

type PutReq struct {
	AppID     string   `json:"appId"`
	UID       string   `json:"uid"`
	ClusterID string   `json:"clusterId"`
	Devices   []string `json:"devices,omitempty"`
}

type DeleteReq struct {
	AppID     string `json:"appId"`
	UID       string `json:"uid"`
	ClusterID string `json:"clusterId"`
}

type ClusterPresence struct {
	ClusterID string   `json:"clusterId"`
	Devices   []string `json:"devices,omitempty"`
	UpdatedAt int64    `json:"updatedAt"`
}

type UserPresence struct {
	AppID              string            `json:"appId"`
	UID                string            `json:"uid"`
	Clusters           []ClusterPresence `json:"clusters"`           // 当前在线集群；全离线可为空
	PreferredClusterID string            `json:"preferredClusterId"` // 最后登录集群；离线投递回退
}

type ClusterInfo struct {
	ClusterID      string `json:"clusterId"`
	Name           string `json:"name"`
	BridgeGRPCAddr string `json:"bridgeGrpcAddr"`
	Status         int    `json:"status"`
}

type apiResp struct {
	Result  int             `json:"result"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (c *Client) Put(ctx context.Context, req PutReq) error {
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/presence", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeOK(resp.Body)
}

func (c *Client) Delete(ctx context.Context, req DeleteReq) error {
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/presence", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeOK(resp.Body)
}

func (c *Client) Get(ctx context.Context, appID string, uids ...string) ([]UserPresence, error) {
	q := url.Values{}
	q.Set("appId", appID)
	for _, u := range uids {
		q.Add("uid", u)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/presence?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var ar apiResp
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, err
	}
	if ar.Result != 0 {
		return nil, fmt.Errorf("presence get result=%d msg=%s", ar.Result, ar.Message)
	}
	var data struct {
		Items []UserPresence `json:"items"`
	}
	if len(ar.Data) > 0 {
		_ = json.Unmarshal(ar.Data, &data)
	}
	return data.Items, nil
}

// ResolvePreferredCluster 选择唯一发送方归属集群。
// 优先 preferredClusterId；否则取在线 clusters 中 UpdatedAt 最新者。
// found=false 表示暂无归属，调用方应重试，不得静默丢弃。
// AllocateGeneratorNode 调用 Presence 预留 Snowflake Node（联邦 init-compose 使用）。
func (c *Client) AllocateGeneratorNode(ctx context.Context, clusterID string, candidate int) (node int, created bool, err error) {
	body, _ := json.Marshal(map[string]int{"candidateNode": candidate})
	u := fmt.Sprintf("%s/internal/clusters/%s/generator-node", c.baseURL, url.PathEscape(clusterID))
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return 0, false, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 0, false, err
	}
	defer resp.Body.Close()
	var ar apiResp
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return 0, false, err
	}
	if ar.Result != 0 {
		return 0, false, fmt.Errorf("allocate generator node result=%d msg=%s", ar.Result, ar.Message)
	}
	var data struct {
		ClusterID     string `json:"clusterId"`
		GeneratorNode int    `json:"generatorNode"`
		Created       bool   `json:"created"`
	}
	if len(ar.Data) > 0 {
		_ = json.Unmarshal(ar.Data, &data)
	}
	return data.GeneratorNode, data.Created, nil
}

func (c *Client) ResolvePreferredCluster(ctx context.Context, appID, uid string) (clusterID string, found bool, err error) {
	items, err := c.Get(ctx, appID, uid)
	if err != nil {
		return "", false, err
	}
	preferred := ""
	var bestOnline string
	var bestAt int64
	for _, it := range items {
		if it.PreferredClusterID != "" {
			preferred = it.PreferredClusterID
		}
		for _, cl := range it.Clusters {
			if cl.ClusterID == "" {
				continue
			}
			if cl.UpdatedAt >= bestAt {
				bestAt = cl.UpdatedAt
				bestOnline = cl.ClusterID
			}
		}
	}
	if preferred != "" {
		return preferred, true, nil
	}
	if bestOnline != "" {
		return bestOnline, true, nil
	}
	return "", false, nil
}

func (c *Client) ListClusters(ctx context.Context) ([]ClusterInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/clusters", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var ar apiResp
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, err
	}
	if ar.Result != 0 {
		return nil, fmt.Errorf("clusters result=%d msg=%s", ar.Result, ar.Message)
	}
	var data struct {
		Clusters []ClusterInfo `json:"clusters"`
	}
	if len(ar.Data) > 0 {
		_ = json.Unmarshal(ar.Data, &data)
	}
	return data.Clusters, nil
}

func decodeOK(r io.Reader) error {
	var ar apiResp
	if err := json.NewDecoder(r).Decode(&ar); err != nil {
		return err
	}
	if ar.Result != 0 {
		return fmt.Errorf("presence api result=%d msg=%s", ar.Result, ar.Message)
	}
	return nil
}
