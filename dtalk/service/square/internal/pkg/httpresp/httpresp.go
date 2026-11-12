package httpresp

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/web3chat/txchat/dtalk/service/square/internal/xerror"
)

func AppOK(c *gin.Context, data any) {
	if data == nil {
		data = gin.H{}
	}
	c.JSON(http.StatusOK, gin.H{"result": 0, "message": "操作成功", "data": data})
}

func AppErr(c *gin.Context, err error) {
	c.JSON(http.StatusOK, gin.H{
		"result":  xerror.CodeOf(err),
		"message": xerror.MessageOf(err),
		"data":    nil,
	})
}

func AdminOK(c *gin.Context, data any) {
	if data == nil {
		data = gin.H{}
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "success", "data": data})
}

func AdminErr(c *gin.Context, err error) {
	code := 500
	if ae, ok := xerror.AsApp(err); ok {
		if ae.Code == xerror.AdminUnauthorized {
			code = 401
		} else if ae.Code == xerror.RateLimited || ae.Code == xerror.AdminLocked || ae.Code == xerror.AdminBadCred || ae.Code == xerror.AdminCaptcha {
			code = 400
		} else if ae.Code == xerror.Forbidden {
			code = 403
		} else if ae.Code == xerror.NotFound {
			code = 404
		} else {
			code = 400
		}
		c.JSON(http.StatusOK, gin.H{"code": code, "message": ae.Message, "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 500, "message": xerror.MessageOf(err), "data": nil})
}
