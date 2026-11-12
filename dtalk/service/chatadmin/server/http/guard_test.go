package http

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestObserveBansAfterRepeatedMisses(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	guard := newIPGuard()
	guard.now = func() time.Time { return now }

	if alert := guard.observe("203.0.113.8", "/api/v1/nope"); alert != nil {
		t.Fatal("第 1 次探测不应拉黑")
	}
	if alert := guard.observe("203.0.113.8", "/wp-admin"); alert != nil {
		t.Fatal("第 2 次探测不应拉黑")
	}
	alert := guard.observe("203.0.113.8", "/console/missing.js")
	if alert == nil {
		t.Fatal("第 3 次探测应拉黑")
	}
	if alert.IP != "203.0.113.8" || alert.Hits != probeLimit {
		t.Fatalf("alert=%+v", alert)
	}
	if !alert.BannedUntil.Equal(now.Add(banDuration)) {
		t.Fatalf("bannedUntil=%s", alert.BannedUntil)
	}
	until, ok := guard.bannedUntil("203.0.113.8")
	if !ok || !until.Equal(alert.BannedUntil) {
		t.Fatalf("ban missing until=%s ok=%v", until, ok)
	}

	now = now.Add(banDuration + time.Second)
	if _, ok := guard.bannedUntil("203.0.113.8"); ok {
		t.Fatal("超过 10 分钟后应解除拉黑")
	}
}

func TestObserveIgnoresBrowserNoiseAndOldHits(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	guard := newIPGuard()
	guard.now = func() time.Time { return now }

	if alert := guard.observe("203.0.113.9", "/favicon.ico"); alert != nil {
		t.Fatal("favicon 不应计数")
	}
	guard.observe("203.0.113.9", "/missing-a")
	guard.observe("203.0.113.9", "/missing-b")
	now = now.Add(probeWindow + time.Second)
	if alert := guard.observe("203.0.113.9", "/missing-c"); alert != nil {
		t.Fatal("窗口过期后的探测应重新计数")
	}
}

func TestProbeMissDoesNotBanBusinessNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	guard := newIPGuard()
	engine := gin.New()
	if err := engine.SetTrustedProxies(nil); err != nil {
		t.Fatal(err)
	}
	engine.Use(guard.middleware())
	engine.NoRoute(notFound)
	engine.GET("/api/v1/groupmanage/known", func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"message": "记录不存在"})
	})

	for i := 0; i < probeLimit; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/groupmanage/known", nil)
		req.RemoteAddr = "203.0.113.10:1234"
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("business 404 status=%d", rec.Code)
		}
	}
	if _, ok := guard.bannedUntil("203.0.113.10"); ok {
		t.Fatal("已注册接口返回的 404 不应拉黑")
	}

	paths := []string{"/api/v1/ghost", "/.env", "/console/not-a-file"}
	for _, path := range paths {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "203.0.113.10:1234"
		req.Header.Set("X-Forwarded-For", "198.51.100.20")
		engine.ServeHTTP(rec, req)
	}
	if _, ok := guard.bannedUntil("198.51.100.20"); ok {
		t.Fatal("伪造的 X-Forwarded-For 不应被拉黑")
	}
	if _, ok := guard.bannedUntil("203.0.113.10"); !ok {
		t.Fatal("真实对端探测后应被拉黑")
	}

	blocked := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ghost", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	engine.ServeHTTP(blocked, req)
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("banned status=%d", blocked.Code)
	}
	body, err := io.ReadAll(blocked.Body)
	if err != nil {
		t.Fatal(err)
	}
	var resp struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Message, "10 分钟") && !strings.Contains(resp.Message, "分钟") {
		t.Fatalf("message=%s", resp.Message)
	}

	alerts := guard.listAlerts()
	if len(alerts) != 1 || alerts[0].IP != "203.0.113.10" {
		t.Fatalf("alerts=%+v", alerts)
	}
}
