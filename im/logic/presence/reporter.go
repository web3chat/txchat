package presence

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Reporter 异步上报用户所在集群到中心 Presence（失败仅打日志，不影响本集群）
type Reporter struct {
	baseURL   string
	clusterID string
	client    *http.Client
	enabled   bool
}

func NewReporter(baseURL, clusterID string, timeout time.Duration) *Reporter {
	enabled := baseURL != "" && clusterID != ""
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &Reporter{
		baseURL:   strings.TrimRight(baseURL, "/"),
		clusterID: clusterID,
		client:    &http.Client{Timeout: timeout},
		enabled:   enabled,
	}
}

func (r *Reporter) Enabled() bool { return r != nil && r.enabled }

func (r *Reporter) AsyncPut(appID, uid string) {
	if !r.Enabled() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), r.client.Timeout)
		defer cancel()
		body, _ := json.Marshal(map[string]interface{}{
			"appId":     appID,
			"uid":       uid,
			"clusterId": r.clusterID,
		})
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, r.baseURL+"/presence", bytes.NewReader(body))
		if err != nil {
			log.Error().Err(err).Str("appId", appID).Str("uid", uid).Str("clusterId", r.clusterID).Msg("presence AsyncPut NewRequest")
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := r.client.Do(req)
		if err != nil {
			log.Error().Err(err).Str("appId", appID).Str("uid", uid).Str("clusterId", r.clusterID).Msg("presence AsyncPut Do")
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			log.Error().Int("status", resp.StatusCode).Str("appId", appID).Str("uid", uid).Str("clusterId", r.clusterID).Msg("presence AsyncPut bad status")
		}
	}()
}

func (r *Reporter) AsyncDelete(appID, uid string) {
	if !r.Enabled() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), r.client.Timeout)
		defer cancel()
		body, _ := json.Marshal(map[string]string{
			"appId":     appID,
			"uid":       uid,
			"clusterId": r.clusterID,
		})
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, r.baseURL+"/presence", bytes.NewReader(body))
		if err != nil {
			log.Error().Err(err).Str("appId", appID).Str("uid", uid).Str("clusterId", r.clusterID).Msg("presence AsyncDelete NewRequest")
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := r.client.Do(req)
		if err != nil {
			log.Error().Err(err).Str("appId", appID).Str("uid", uid).Str("clusterId", r.clusterID).Msg("presence AsyncDelete Do")
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			log.Error().Int("status", resp.StatusCode).Str("appId", appID).Str("uid", uid).Msg("presence AsyncDelete bad status")
		}
	}()
}
