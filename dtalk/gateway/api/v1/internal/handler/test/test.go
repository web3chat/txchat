package handler

import (
	"github.com/gin-gonic/gin"
	logic "github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/logic/test"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/svc"
	"github.com/web3chat/txchat/dtalk/pkg/api"
)

func GetHelloWord(ctx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		l := logic.NewTestLogic(c.Request.Context(), ctx)
		resp, err := l.GetHelloWorld()
		c.Set(api.ReqResult, resp)
		c.Set(api.ReqError, err)
	}
}
