package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/inconshreveable/log15"
	"github.com/web3chat/txchat/dtalk/pkg/api"
	xerror "github.com/web3chat/txchat/dtalk/pkg/error"
	"github.com/web3chat/txchat/dtalk/service/federation/service"
)

var (
	svc *service.Service
	log = log15.New("module", "federation/http")
)

func Init(s *service.Service, addr string) *http.Server {
	if addr == "" {
		return nil
	}
	svc = s
	engine := gin.New()
	engine.Use(gin.Recovery())
	root := engine.Group("/", api.RespMiddleWare())
	root.POST("/admin/federation/enabled", SetEnabled)
	root.GET("/admin/federation/enabled", GetEnabled)

	srv := &http.Server{Addr: addr, Handler: engine}
	go func() {
		log.Info("federation admin http", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("ListenAndServe", "err", err)
		}
	}()
	return srv
}

type enabledReq struct {
	Enabled bool `json:"enabled"`
}

func SetEnabled(c *gin.Context) {
	var req enabledReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Set(api.ReqError, xerror.NewError(xerror.ParamsError))
		return
	}
	svc.SetEnabled(req.Enabled)
	log.Info("federation enabled changed", "enabled", req.Enabled)
	c.Set(api.ReqResult, map[string]interface{}{"enabled": svc.Enabled()})
	c.Set(api.ReqError, nil)
}

func GetEnabled(c *gin.Context) {
	c.Set(api.ReqResult, map[string]interface{}{"enabled": svc.Enabled()})
	c.Set(api.ReqError, nil)
}
