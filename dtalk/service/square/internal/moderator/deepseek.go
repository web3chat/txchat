package moderator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/web3chat/txchat/dtalk/service/square/internal/config"
	"github.com/web3chat/txchat/dtalk/service/square/internal/entity"
)

type Result struct {
	Verdict     string   `json:"verdict"`
	Categories  []string `json:"categories"`
	Reason      string   `json:"reason"`
	Unavailable bool     `json:"-"`
}

type Moderators interface {
	Moderate(ctx context.Context, text string, imageURLs []string) (Result, int, error)
}

type DeepSeek struct {
	cfg    config.DeepSeekConfig
	client *http.Client
}

func NewDeepSeek(cfg config.DeepSeekConfig) *DeepSeek {
	cfg.Model = resolveTextModel(cfg.Model)
	return &DeepSeek{
		cfg: cfg,
		client: &http.Client{
			Timeout: visionTimeout,
		},
	}
}

const systemPrompt = `你是内容安全审核员。只输出 JSON，不要 markdown。
JSON 格式：{"verdict":"pass|reject|review","categories":[],"reason":""}

只审用户给出的文字。不要根据图片、链接或文件名推断画面内容。

reject 仅用于「语义明确且明显违规」，必须给出 categories：
- politics：点名辱骂、煽动攻击在世政治人物，或煽动颠覆
- porn：明确色情描写或性交易
- illegal：明确教唆或实施违法犯罪（毒品、诈骗、武器等）

pass 是默认结论：日常闲聊、看不懂、语义不清、没有明显违规、时政新闻、历史讨论、轻微牢骚、擦边但未达色情。
宁松勿严：拿不准就 pass，不要因为「可能」「感觉像」「影射」就拦。

review 极少使用：仅当高度疑似色情或违法、但还差一步才写明确时，categories 必须含 porn 或 illegal。
categories 仅可含 politics、porn、illegal。`

const (
	visionTimeout  = 30 * time.Second
	maxImageURLLen = 8192
	imageDetailLow = "low"
)

func (d *DeepSeek) Moderate(ctx context.Context, text string, imageURLs []string) (Result, int, error) {
	if strings.TrimSpace(d.cfg.APIKey) == "" {
		slog.Warn("deepseek api key empty, return review")
		return Result{Verdict: entity.VerdictReview, Reason: "moderator unavailable", Unavailable: true}, 0, nil
	}
	urls, dropped := httpImageURLs(imageURLs)
	if dropped > 0 {
		slog.Warn("skip image url for moderation", "dropped", dropped, "kept", len(urls))
	}
	model := strings.TrimSpace(d.cfg.Model)
	timeout := d.cfg.Timeout
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	if len(urls) > 0 {
		model = visionModel(d.cfg)
		if timeout < visionTimeout {
			timeout = visionTimeout
		}
	}
	body := map[string]any{
		"model": model,
		"messages": []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent(text, urls)},
		},
		"temperature": 0,
		"thinking":    map[string]string{"type": "disabled"},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return Result{}, 0, err
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, strings.TrimRight(d.cfg.BaseURL, "/")+"/v1/chat/completions", strings.NewReader(string(raw)))
	if err != nil {
		return Result{}, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.cfg.APIKey)
	if len(urls) > 0 {
		slog.Info("deepseek moderate images", "model", model, "images", len(urls), "dropped", dropped)
	}
	resp, err := d.client.Do(req)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		slog.Error("deepseek request failed", "model", model, "images", len(urls), "err", err, "latency_ms", latency)
		return Result{Verdict: entity.VerdictReview, Reason: "moderator timeout", Unavailable: true}, latency, nil
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		snippet := httpErrorSnippet(respBody, d.cfg.APIKey)
		slog.Error("deepseek http error", "model", model, "images", len(urls), "status", resp.StatusCode, "latency_ms", latency, "body", snippet)
		return Result{Verdict: entity.VerdictReview, Reason: fmt.Sprintf("moderator http %d", resp.StatusCode), Unavailable: true}, latency, nil
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil || len(parsed.Choices) == 0 {
		slog.Error("deepseek parse failed", "model", model, "images", len(urls), "err", err)
		return Result{Verdict: entity.VerdictReview, Reason: "moderator parse error", Unavailable: true}, latency, nil
	}
	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	var r Result
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &r); err != nil {
		slog.Error("deepseek json verdict failed", "model", model, "images", len(urls), "err", err)
		return Result{Verdict: entity.VerdictReview, Reason: "moderator json error", Unavailable: true}, latency, nil
	}
	before := r.Verdict
	r = sanitizeResult(r)
	if dropped > 0 {
		r.Unavailable = true
	}
	if before != r.Verdict {
		slog.Info("moderation sanitized", "from", before, "to", r.Verdict, "categories", r.Categories, "images", len(urls))
	}
	return r, latency, nil
}

type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

func resolveTextModel(model string) string {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "", "deepseek-chat":
		if strings.EqualFold(strings.TrimSpace(model), "deepseek-chat") {
			slog.Warn("deepseek text model retired, using deepseek-flash", "configured", model)
		}
		return "deepseek-flash"
	default:
		return strings.TrimSpace(model)
	}
}

func httpErrorSnippet(body []byte, secret string) string {
	s := strings.Join(strings.Fields(string(body)), " ")
	if secret != "" {
		s = strings.ReplaceAll(s, secret, "[redacted]")
	}
	const limit = 400
	r := []rune(s)
	if len(r) > limit {
		return string(r[:limit])
	}
	return s
}

func visionModel(cfg config.DeepSeekConfig) string {
	if m := strings.TrimSpace(cfg.VisionModel); m != "" {
		return m
	}
	return "deepseek-flash"
}

func httpImageURLs(raw []string) (kept []string, dropped int) {
	for _, item := range raw {
		u := strings.TrimSpace(item)
		if u == "" {
			continue
		}
		if len(u) > maxImageURLLen || (!strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://")) {
			dropped++
			continue
		}
		kept = append(kept, u)
	}
	return kept, dropped
}

func userContent(text string, urls []string) any {
	if len(urls) == 0 {
		return "请只根据下面文字判断。\n正文：\n" + text
	}
	parts := make([]map[string]any, 0, 1+len(urls))
	parts = append(parts, map[string]any{
		"type": "text",
		"text": "请同时根据正文和附图判断。\n正文：\n" + text,
	})
	for _, u := range urls {
		parts = append(parts, map[string]any{
			"type": "image_url",
			"image_url": map[string]string{
				"url":    u,
				"detail": imageDetailLow,
			},
		})
	}
	return parts
}

// HoldImageFailure 在默认策略把「没审成」放行之后，把带图请求拉回人工。文字请求不受影响。
func HoldImageFailure(r Result, hasImages bool) Result {
	if !hasImages || !r.Unavailable || r.Verdict != entity.VerdictPass {
		return r
	}
	r.Verdict = entity.VerdictReview
	if strings.TrimSpace(r.Reason) == "" {
		r.Reason = "图片未完成审核，转入人工"
	}
	return r
}

func sanitizeResult(r Result) Result {
	r.Verdict = strings.ToLower(strings.TrimSpace(r.Verdict))
	cats := make([]string, 0, len(r.Categories))
	seen := map[string]struct{}{}
	for _, c := range r.Categories {
		c = strings.ToLower(strings.TrimSpace(c))
		if c != "politics" && c != "porn" && c != "illegal" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		cats = append(cats, c)
	}
	r.Categories = cats
	return r
}

func LoosenResult(r Result) Result {
	return loosenResult(r)
}

func TightenResult(r Result) Result {
	return tightenResult(r)
}

func loosenResult(r Result) Result {
	r = sanitizeResult(r)
	switch r.Verdict {
	case entity.VerdictPass, entity.VerdictReject:
		return r
	case entity.VerdictReview:
		keep := false
		for _, c := range r.Categories {
			if c == "porn" || c == "illegal" {
				keep = true
				break
			}
		}
		if keep {
			return r
		}
		r.Verdict = entity.VerdictPass
		if strings.TrimSpace(r.Reason) == "" {
			r.Reason = "无明确违规，自动放行"
		}
		return r
	default:
		r.Verdict = entity.VerdictPass
		if strings.TrimSpace(r.Reason) == "" {
			r.Reason = "结论无法识别，按放行处理"
		}
		return r
	}
}

func tightenResult(r Result) Result {
	r = sanitizeResult(r)
	switch r.Verdict {
	case entity.VerdictPass, entity.VerdictReject, entity.VerdictReview:
		return r
	default:
		r.Verdict = entity.VerdictReview
		if strings.TrimSpace(r.Reason) == "" {
			r.Reason = "结论无法识别，转入人工审核"
		}
		return r
	}
}

func CategoriesJSON(cats []string) []byte {
	b, err := json.Marshal(cats)
	if err != nil {
		return []byte("[]")
	}
	return b
}

func StatusOf(verdict string) int16 {
	switch verdict {
	case entity.VerdictPass:
		return entity.StatusVisible
	case entity.VerdictReject:
		return entity.StatusAIReject
	default:
		return entity.StatusPending
	}
}

func ResultError(verdict string) error {
	switch verdict {
	case entity.VerdictReject:
		return fmt.Errorf("illegal")
	case entity.VerdictReview:
		return fmt.Errorf("pending")
	default:
		return nil
	}
}
