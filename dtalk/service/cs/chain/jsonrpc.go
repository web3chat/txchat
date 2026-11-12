// Package chain 封装对 Chain33 JSON-RPC（与 ChatPro-Android 合约域名 POST "." 相同协议）的调用。
package chain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

type rpcReq struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"` // 无参时为 []
}

// Chain33 节点返回的 error 可能是对象 {code,message}，也可能是字符串（如 gRPC 包装的 "rpc error: ... ErrNoBalance"）
type rpcResp struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   json.RawMessage `json:"error"`
}

func rpcErrorText(errRaw json.RawMessage) string {
	if len(errRaw) == 0 {
		return ""
	}
	s := strings.TrimSpace(string(errRaw))
	if s == "" || s == "null" {
		return ""
	}
	var asString string
	if json.Unmarshal(errRaw, &asString) == nil && asString != "" {
		return asString
	}
	var obj struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    string `json:"data,omitempty"`
	}
	if json.Unmarshal(errRaw, &obj) == nil && (obj.Message != "" || obj.Code != 0) {
		if obj.Message != "" {
			return fmt.Sprintf("%s (code=%d)", obj.Message, obj.Code)
		}
		return fmt.Sprintf("code=%d", obj.Code)
	}
	return s
}

// Call 向 chain RPC 节点发送一条 JSON-RPC 2.0 请求。
// 无额外参数时（如 Chain33.GetBlockHeight）传零个 args，对应 "params":[]；
// 有一个参数时传一个元素，对应 "params":[obj]。
func Call(chainRPC string, method string, params ...interface{}) (json.RawMessage, error) {
	if chainRPC == "" {
		return nil, fmt.Errorf("empty chain RPC URL")
	}
	p := []interface{}{}
	if len(params) > 0 {
		p = append(p, params...)
	}
	body := rpcReq{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  p,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, chainRPC, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	cli := &http.Client{Timeout: defaultTimeout}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var out rpcResp
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("parse rpc response: %w body=%s", err, string(b))
	}
	if msg := rpcErrorText(out.Error); msg != "" {
		return nil, fmt.Errorf("%s", msg)
	}
	return out.Result, nil
}

// GetBlockHeight 探测链是否可用（Chain33.GetBlockHeight）。
func GetBlockHeight(chainRPC string) (int64, error) {
	res, err := Call(chainRPC, "Chain33.GetBlockHeight")
	if err != nil {
		return 0, err
	}
	var h int64
	if err := json.Unmarshal(res, &h); err != nil {
		var s string
		if err2 := json.Unmarshal(res, &s); err2 == nil {
			s = strings.Trim(strings.TrimSpace(s), `"`)
			v, err3 := strconv.ParseInt(s, 10, 64)
			if err3 == nil {
				return v, nil
			}
		}
		return 0, fmt.Errorf("unmarshal height: %w raw=%s", err, string(res))
	}
	return h, nil
}
