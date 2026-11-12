package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/web3chat/txchat/dtalk/service/group/model"
	"github.com/web3chat/txchat/dtalk/service/group/service"
)

// WhitelistHandler 白名单管理接口
type WhitelistHandler struct {
	svc *service.Service
}

// NewWhitelistHandler 创建白名单处理器
func NewWhitelistHandler(svc *service.Service) *WhitelistHandler {
	return &WhitelistHandler{
		svc: svc,
	}
}

// AddWhitelistRequest 添加白名单请求
type AddWhitelistRequest struct {
	Address    string `json:"address" binding:"required"`
	Remark     string `json:"remark"`
	MaxMembers int32  `json:"max_members"`
	CreatedBy  string `json:"created_by"`
}

// UpdateWhitelistRequest 更新白名单请求
type UpdateWhitelistRequest struct {
	Address    string `json:"address" binding:"required"`
	Remark     string `json:"remark"`
	Status     int32  `json:"status"`
	MaxMembers int32  `json:"max_members"`
}

// GetWhitelistListRequest 查询白名单列表请求
type GetWhitelistListRequest struct {
	Offset int `form:"offset"`
	Limit  int `form:"limit"`
}

// AddWhitelist 添加白名单记录
func (h *WhitelistHandler) AddWhitelist(c *gin.Context) {
	var req AddWhitelistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"result":  -1,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	whitelistInfo := &model.WhitelistInfo{
		Address:    req.Address,
		Remark:     req.Remark,
		Status:     1, // 默认启用
		MaxMembers: req.MaxMembers,
		CreatedBy:  req.CreatedBy,
	}

	err := h.svc.WhitelistDAO.AddWhitelist(c.Request.Context(), whitelistInfo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"result":  -1,
			"message": "添加白名单记录失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result":  0,
		"message": "添加成功",
		"data":    whitelistInfo,
	})
}

// UpdateWhitelist 更新白名单记录
func (h *WhitelistHandler) UpdateWhitelist(c *gin.Context) {
	var req UpdateWhitelistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"result":  -1,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	whitelistInfo := &model.WhitelistInfo{
		Address:    req.Address,
		Remark:     req.Remark,
		Status:     req.Status,
		MaxMembers: req.MaxMembers,
	}

	err := h.svc.WhitelistDAO.UpdateWhitelist(c.Request.Context(), whitelistInfo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"result":  -1,
			"message": "更新白名单记录失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result":  0,
		"message": "更新成功",
		"data":    whitelistInfo,
	})
}

// DeleteWhitelist 删除白名单记录（软删除）
func (h *WhitelistHandler) DeleteWhitelist(c *gin.Context) {
	address := c.Param("address")
	if address == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"result":  -1,
			"message": "地址参数不能为空",
		})
		return
	}

	err := h.svc.WhitelistDAO.DeleteWhitelist(c.Request.Context(), address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"result":  -1,
			"message": "删除白名单记录失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result":  0,
		"message": "删除成功",
	})
}

// GetWhitelistList 查询白名单列表
func (h *WhitelistHandler) GetWhitelistList(c *gin.Context) {
	var req GetWhitelistListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"result":  -1,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 设置默认值
	if req.Offset < 0 {
		req.Offset = 0
	}
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 10
	}

	list, err := h.svc.WhitelistDAO.GetWhitelistList(c.Request.Context(), req.Offset, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"result":  -1,
			"message": "查询白名单列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result":  0,
		"message": "查询成功",
		"data": gin.H{
			"list":   list,
			"offset": req.Offset,
			"limit":  req.Limit,
			"total":  len(list),
		},
	})
}

// CheckWhitelist 检查地址是否在白名单中
func (h *WhitelistHandler) CheckWhitelist(c *gin.Context) {
	address := c.Query("address")
	if address == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"result":  -1,
			"message": "地址参数不能为空",
		})
		return
	}

	whitelistInfo, err := h.svc.WhitelistDAO.CheckWhitelistWithLimit(c.Request.Context(), address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"result":  -1,
			"message": "检查白名单失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result":  0,
		"message": "检查成功",
		"data":    whitelistInfo,
	})
}

// RegisterRoutes 注册白名单管理路由
func (h *WhitelistHandler) RegisterRoutes(r *gin.Engine) {
	whitelist := r.Group("/whitelist")
	{
		whitelist.POST("/add", h.AddWhitelist)                  // 添加白名单
		whitelist.PUT("/update", h.UpdateWhitelist)             // 更新白名单
		whitelist.DELETE("/delete/:address", h.DeleteWhitelist) // 删除白名单
		whitelist.GET("/list", h.GetWhitelistList)              // 查询白名单列表
		whitelist.GET("/check", h.CheckWhitelist)               // 检查白名单
	}
}
