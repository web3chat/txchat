package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/web3chat/txchat/dtalk/service/cs/config"
)

type dsChatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type dsRequest struct {
	Model       string        `json:"model"`
	Messages    []dsChatMsg   `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type dsResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func deepSeekChat(ctx context.Context, cfg *config.OfflineAIConfig, log log15.Logger, systemPrompt string, history []dsChatMsg) (string, error) {
	if cfg == nil || strings.TrimSpace(cfg.DeepSeekAPIKey) == "" {
		return "", fmt.Errorf("DeepSeekAPIKey 未配置")
	}
	base := strings.TrimSpace(cfg.DeepSeekBaseURL)
	if base == "" {
		base = "https://api.deepseek.com"
	}
	base = strings.TrimSuffix(base, "/")
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = "deepseek-chat"
	}

	msgs := make([]dsChatMsg, 0, len(history)+1)
	msgs = append(msgs, dsChatMsg{Role: "system", Content: systemPrompt})
	msgs = append(msgs, history...)

	body := dsRequest{
		Model:       model,
		Messages:    msgs,
		Temperature: 0.45,
		MaxTokens:   1024,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	url := base + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.DeepSeekAPIKey)

	cli := &http.Client{Timeout: 120 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		log.Warn("DeepSeek 请求失败", "url", url, "err", err)
		return "", err
	}
	defer resp.Body.Close()

	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Warn("DeepSeek HTTP 非成功", "status", resp.StatusCode, "body_prefix", truncBody(rb, 400))
		return "", fmt.Errorf("deepseek http %d: %s", resp.StatusCode, truncBody(rb, 200))
	}

	var dr dsResponse
	if err := json.Unmarshal(rb, &dr); err != nil {
		return "", fmt.Errorf("deepseek json: %w", err)
	}
	if dr.Error != nil && dr.Error.Message != "" {
		return "", fmt.Errorf("deepseek api error: %s", dr.Error.Message)
	}
	if len(dr.Choices) == 0 || strings.TrimSpace(dr.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("deepseek empty choices")
	}
	out := strings.TrimSpace(dr.Choices[0].Message.Content)
	log.Info("DeepSeek 回复成功", "chars", len(out))
	return out, nil
}

func truncBody(b []byte, n int) string {
	s := string(b)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
