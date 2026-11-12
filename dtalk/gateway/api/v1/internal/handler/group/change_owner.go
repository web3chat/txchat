package group

import (
	"github.com/gin-gonic/gin"
	logic "github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/logic/group"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/svc"
	"github.com/web3chat/txchat/dtalk/gateway/api/v1/internal/types"
	"github.com/web3chat/txchat/dtalk/pkg/api"
	xerror "github.com/web3chat/txchat/dtalk/pkg/error"
	"github.com/web3chat/txchat/dtalk/pkg/util"
)

// ChangeOwnerHandler
// @Summary 转让群
// @Author chy@33.cn
// @Tags group 群动作
// @Param FZM-SIGNATURE	header string true "MOCK"
// @Param data body types.ChangeOwnerReq false "body"
// @Success 200 {object} types.GeneralResp{data=types.ChangeOwnerResp}
// @Router	/group/app/change-owner [post]
func ChangeOwnerHandler(ctx *svc.ServiceContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := &types.ChangeOwnerReq{}
		err := c.ShouldBind(req)
		if err != nil {
			c.Set(api.ReqError, xerror.NewError(xerror.ParamsError).SetExtMessage(err.Error()))
			return
		}

		if req.Id == 0 && req.IdStr == "" {
			c.Set(api.ReqError, xerror.NewError(xerror.ParamsError))
			return
		}

		if req.IdStr != "" {
			req.Id = util.ToInt64(req.IdStr)
		}

		l := logic.NewGroupLogic(c, ctx)
		res, err := l.ChangeOwner(req)

		c.Set(api.ReqResult, res)
		c.Set(api.ReqError, err)
	}
}
