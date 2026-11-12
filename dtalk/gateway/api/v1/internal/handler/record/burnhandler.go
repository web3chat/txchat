package record

import (
	"github.com/gin-gonic/gin"

	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/logic/record"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/model"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/svc"
	"github.com/web3chat/txchat/dtalk/pkg/api"
	xerror "github.com/web3chat/txchat/dtalk/pkg/error"
)

// BurnConfirmHandler
// @Summary 阅后即焚确认焚毁
// @Tags record 消息模块
// @Accept json
// @Produce json
// @Param FZM-SIGNATURE header string true "鉴权签名"
// @Param data body model.BurnConfirmReq true "请求体"
// @Success 200 {object} model.GeneralResponse{data=model.BurnConfirmResp}
// @Router /app/record/burn-confirm [post]
func BurnConfirmHandler(ctx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := &model.BurnConfirmReq{}
		if err := c.ShouldBind(req); err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.ParamsError).SetExtMessage(err.Error()))
			return
		}
		userId, ok := c.Get(api.Address)
		if !ok {
			c.Set(api.ReqError, xerror.NewError(xerror.SignatureInvalid))
			return
		}
		l := record.NewLogic(c.Request.Context(), ctx)
		data, err := l.BurnConfirm(userId.(string), req.TargetId, req.LogId, req.Type)
		c.Set(api.ReqResult, data)
		c.Set(api.ReqError, err)
	}
}
