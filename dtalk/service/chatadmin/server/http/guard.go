package http

import (
	"fmt"
	"html"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/web3chat/txchat/dtalk/service/chatadmin/model/types"
)

const (
	probeWindow   = time.Minute
	probeLimit    = 3
	banDuration   = 10 * time.Minute
	maxProbeNotes = 40
)

// securityGuard 记录对不存在页面、接口的探测，达到次数后拉黑来源 IP。
var securityGuard = newIPGuard()

type probeHit struct {
	path string
	at   time.Time
}

type securityAlert struct {
	ID          int64     `json:"id"`
	IP          string    `json:"ip"`
	Path        string    `json:"path"`
	Hits        int       `json:"hits"`
	BannedUntil time.Time `json:"bannedUntil"`
	RemainSec   int       `json:"remainSec"`
	Message     string    `json:"message"`
}

type ipGuard struct {
	mu     sync.Mutex
	now    func() time.Time
	hits   map[string][]probeHit
	bans   map[string]time.Time
	alerts []securityAlert
	nextID int64
}

func newIPGuard() *ipGuard {
	return &ipGuard{
		now:  time.Now,
		hits: make(map[string][]probeHit),
		bans: make(map[string]time.Time),
	}
}

func (g *ipGuard) middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		ip := c.ClientIP()
		if until, ok := g.bannedUntil(ip); ok {
			g.rejectBanned(c, ip, until)
			return
		}
		c.Next()
		if !isProbeMiss(c) {
			return
		}
		if alert := g.observe(ip, c.Request.URL.Path); alert != nil {
			log.Warn().
				Str("ip", alert.IP).
				Str("method", c.Request.Method).
				Str("path", alert.Path).
				Int("hits", alert.Hits).
				Time("banned_until", alert.BannedUntil).
				Msg("探测不存在的接口，已拉黑 IP")
		}
	}
}

// isProbeMiss 只把“路由或静态文件不存在”算探测。
// 已注册接口自己返回的 404（例如地址不在白名单）不计数。
func isProbeMiss(c *gin.Context) bool {
	if c.GetBool("routeMiss") {
		return true
	}
	if c.Writer.Status() != http.StatusNotFound {
		return false
	}
	return !strings.HasPrefix(c.Request.URL.Path, "/api/")
}

func notFound(c *gin.Context) {
	c.Set("routeMiss", true)
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		c.JSON(http.StatusNotFound, types.APIResponse{Code: http.StatusNotFound, Message: "接口不存在"})
		return
	}
	c.String(http.StatusNotFound, "页面不存在")
}

func (g *ipGuard) rejectBanned(c *gin.Context, ip string, until time.Time) {
	remain := int(time.Until(until).Seconds())
	if remain < 1 {
		remain = 1
	}
	log.Info().
		Str("ip", ip).
		Str("method", c.Request.Method).
		Str("path", c.Request.URL.Path).
		Int("remain_sec", remain).
		Msg("拒绝已拉黑 IP")
	msg := fmt.Sprintf("该 IP 因探测不存在的接口已被限制访问，请 %d 分钟后再试", (remain+59)/60)
	if strings.HasPrefix(c.Request.URL.Path, "/api/") || wantsJSON(c) {
		c.AbortWithStatusJSON(http.StatusForbidden, types.APIResponse{Code: http.StatusForbidden, Message: msg})
		return
	}
	page := "<!DOCTYPE html><html lang=\"zh-CN\"><meta charset=\"utf-8\"><title>访问受限</title><body><p>" +
		html.EscapeString(msg) + "</p></body></html>"
	c.Data(http.StatusForbidden, "text/html; charset=utf-8", []byte(page))
	c.Abort()
}

func wantsJSON(c *gin.Context) bool {
	accept := c.GetHeader("Accept")
	return strings.Contains(accept, "application/json")
}

func (g *ipGuard) bannedUntil(ip string) (time.Time, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	until, ok := g.bans[ip]
	if !ok {
		return time.Time{}, false
	}
	if !g.now().Before(until) {
		delete(g.bans, ip)
		return time.Time{}, false
	}
	return until, true
}

// observe 记录一次不存在路径。窗口内累计达到阈值则拉黑，并返回提醒。
func (g *ipGuard) observe(ip, path string) *securityAlert {
	if ip == "" || skipProbePath(path) {
		return nil
	}
	path = clipPath(path)
	now := g.now()

	g.mu.Lock()
	defer g.mu.Unlock()

	if until, ok := g.bans[ip]; ok && now.Before(until) {
		return nil
	}
	cut := now.Add(-probeWindow)
	kept := make([]probeHit, 0, len(g.hits[ip])+1)
	for _, hit := range g.hits[ip] {
		if hit.at.After(cut) {
			kept = append(kept, hit)
		}
	}
	kept = append(kept, probeHit{path: path, at: now})
	if len(kept) > maxProbeNotes {
		kept = kept[len(kept)-maxProbeNotes:]
	}
	g.hits[ip] = kept
	if len(kept) < probeLimit {
		return nil
	}

	until := now.Add(banDuration)
	g.bans[ip] = until
	delete(g.hits, ip)
	g.nextID++
	alert := securityAlert{
		ID:          g.nextID,
		IP:          ip,
		Path:        path,
		Hits:        len(kept),
		BannedUntil: until,
		RemainSec:   int(banDuration.Seconds()),
		Message:     fmt.Sprintf("IP %s 在 1 分钟内探测不存在的接口 %d 次（最近 %s），已拉黑 10 分钟", ip, len(kept), path),
	}
	g.alerts = append(g.alerts, alert)
	if len(g.alerts) > maxProbeNotes {
		g.alerts = g.alerts[len(g.alerts)-maxProbeNotes:]
	}
	copied := alert
	return &copied
}

func (g *ipGuard) listAlerts() []securityAlert {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	out := make([]securityAlert, 0, len(g.alerts))
	for _, alert := range g.alerts {
		item := alert
		remain := int(alert.BannedUntil.Sub(now).Seconds())
		if remain < 0 {
			remain = 0
		}
		item.RemainSec = remain
		out = append(out, item)
	}
	return out
}

func skipProbePath(path string) bool {
	switch path {
	case "/favicon.ico", "/robots.txt":
		return true
	default:
		return false
	}
}

func clipPath(path string) string {
	if len(path) > 180 {
		return path[:180]
	}
	return path
}

func listSecurityAlerts(c *gin.Context) {
	c.JSON(http.StatusOK, types.APIResponse{
		Code:    http.StatusOK,
		Message: "success",
		Data:    gin.H{"alerts": securityGuard.listAlerts()},
	})
}
