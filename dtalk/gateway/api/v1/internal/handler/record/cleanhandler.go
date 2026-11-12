package record

import (
	"github.com/gin-gonic/gin"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/logic/record"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/model"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/svc"
	"github.com/web3chat/txchat/dtalk/pkg/api"
	xerror "github.com/web3chat/txchat/dtalk/pkg/error"
)

// CleanPrivateChat
// @Summary      手动删除私聊记录
// @Tags         record 消息模块
// @Accept       json
// @Produce      json
// @Param        FZM-SIGNATURE  header  string                    true  "鉴权签名"
// @Param        data           body    model.CleanPrivateChatReq true  "请求体"
// @Success      200            {object} model.GeneralResponse{data=model.CleanPrivateChatResp}
// @Router       /app/record/clean-private [post]
func CleanPrivateChatHandler(ctx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := &model.CleanPrivateChatReq{}
		if err := c.ShouldBind(req); err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.ParamsError).SetExtMessage(err.Error()))
			return
		}

		userId, ok := c.Get(api.Address)
		if !ok {
			c.Set(api.ReqError, xerror.NewError(xerror.SignatureInvalid))
			return
		}
		req.OperatorID = userId.(string)

		l := record.NewLogic(c.Request.Context(), ctx)
		data, err := l.CleanPrivateChat(req)
		c.Set(api.ReqResult, data)
		c.Set(api.ReqError, err)
	}
}

// SetAutoDeleteConfig
// @Summary      设置自动删除配置
// @Tags         record 消息模块
// @Accept       json
// @Produce      json
// @Param        FZM-SIGNATURE  header  string                      true  "鉴权签名"
// @Param        data           body    model.SetAutoDeleteConfigReq true  "请求体"
// @Success      200            {object} model.GeneralResponse{data=model.AutoDeleteConfigResp}
// @Router       /app/record/auto-delete/set [post]
func SetAutoDeleteConfigHandler(ctx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := &model.SetAutoDeleteConfigReq{}
		if err := c.ShouldBind(req); err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.ParamsError).SetExtMessage(err.Error()))
			return
		}

		userId, ok := c.Get(api.Address)
		if !ok {
			c.Set(api.ReqError, xerror.NewError(xerror.SignatureInvalid))
			return
		}
		req.OperatorID = userId.(string)

		l := record.NewLogic(c.Request.Context(), ctx)
		data, err := l.SetAutoDeleteConfig(req)
		c.Set(api.ReqResult, data)
		c.Set(api.ReqError, err)
	}
}

// GetAutoDeleteConfig
// @Summary      查询自动删除配置
// @Tags         record 消息模块
// @Accept       json
// @Produce      json
// @Param        FZM-SIGNATURE  header  string                      true  "鉴权签名"
// @Param        data           body    model.GetAutoDeleteConfigReq true  "请求体"
// @Success      200            {object} model.GeneralResponse{data=model.AutoDeleteConfigResp}
// @Router       /app/record/auto-delete/get [post]
func GetAutoDeleteConfigHandler(ctx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := &model.GetAutoDeleteConfigReq{}
		if err := c.ShouldBind(req); err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.ParamsError).SetExtMessage(err.Error()))
			return
		}

		userId, ok := c.Get(api.Address)
		if !ok {
			c.Set(api.ReqError, xerror.NewError(xerror.SignatureInvalid))
			return
		}
		req.OperatorID = userId.(string)

		l := record.NewLogic(c.Request.Context(), ctx)
		data, err := l.GetAutoDeleteConfig(req)
		c.Set(api.ReqResult, data)
		c.Set(api.ReqError, err)
	}
}
