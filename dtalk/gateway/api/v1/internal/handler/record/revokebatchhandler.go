package record

import (
	"github.com/gin-gonic/gin"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/logic/record"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/model"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/svc"
	"github.com/web3chat/txchat/dtalk/pkg/api"
	xerror "github.com/web3chat/txchat/dtalk/pkg/error"
)

// RevokeBatchHandler
// @Summary 批量撤回消息
// @Description 一次请求撤回多条消息，支持部分成功
// @Author bigsea
// @Tags record 消息模块
// @Accept       json
// @Produce      json
// @Param FZM-SIGNATURE	header string true "MOCK"
// @Param data body model.RevokeBatchMsgReq true "body"
// @Success 200 {object} model.GeneralResponse{data=model.RevokeBatchMsgResp}
// @Router	/app/record/revoke-batch [post]
func RevokeBatchHandler(ctx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := &model.RevokeBatchMsgReq{}
		if err := c.ShouldBind(req); err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.ParamsError).SetExtMessage(err.Error()))
			return
		}

		operator := c.MustGet(api.Address).(string)
		l := record.NewLogic(c.Request.Context(), ctx)
		data := l.RevokeBatch(operator, req)
		c.Set(api.ReqResult, data)
		c.Set(api.ReqError, nil)
	}
}
