package http

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/web3chat/txchat/dtalk/service/chatadmin/model/biz"
	"github.com/web3chat/txchat/dtalk/service/chatadmin/model/types"
)

// getConfig 获取服务配置信息
// @Summary 获取服务配置信息
// @Description 获取服务功能开关等配置信息，包括当前用户的管理员级别
// @Tags 系统配置
// @Accept json
// @Produce json
// @Success 200 {object} types.APIResponse{data=map[string]interface{}}
// @Router /api/v1/groupmanage/config [get]
func getConfig(c *gin.Context) {
	ctx := context.Background()

	// 获取当前用户的管理员级别
	adminLevel := int32(-1) // 默认值：未找到
	adminAddress, exists := c.Get("adminAddress")
	if exists {
		if address, ok := adminAddress.(string); ok {
			level, err := svc.GetAdminLevel(ctx, address)
			if err == nil {
				adminLevel = level
			}
		}
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "success",
		Data: map[string]interface{}{
			"adminLevel": adminLevel, // 0=普通管理员, 1=超级管理员, -1=未找到或已禁用
		},
	})
}

// getGroupList 获取群组列表
// @Summary 获取群组列表
// @Description 分页获取群组列表，支持关键词搜索和状态筛选
// @Tags 群组管理
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(10)
// @Param keyword query string false "搜索关键词"
// @Param status query int false "群组状态" Enums(0,1,2)
// @Success 200 {object} types.APIResponse{data=biz.GroupListResponse}
// @Router /api/v1/groupmanage/groups/list [get]
func getGroupList(c *gin.Context) {
	ctx := context.Background()

	// 解析查询参数
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("pageSize", "10"), 10, 32)
	keyword := c.Query("keyword")
	statusStr := c.Query("status")

	var status int32 = -1
	if statusStr != "" {
		if s, err := strconv.ParseInt(statusStr, 10, 32); err == nil {
			status = int32(s)
		}
	}

	req := &biz.GroupListRequest{
		Page:     int32(page),
		PageSize: int32(pageSize),
		Keyword:  keyword,
		Status:   status,
	}

	resp, err := svc.GetGroupList(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "获取群组列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "success",
		Data:    resp,
	})
}

// getGroupInfo 获取群组信息
// @Summary 获取群组信息
// @Description 根据群组ID获取群组详细信息
// @Tags 群组管理
// @Accept json
// @Produce json
// @Param id path int true "群组ID"
// @Success 200 {object} types.APIResponse{data=biz.GroupInfo}
// @Router /api/v1/groupmanage/groups/{id} [get]
func getGroupInfo(c *gin.Context) {
	ctx := context.Background()

	groupIdStr := c.Param("id")
	groupId := new(big.Int)
	_, ok := groupId.SetString(groupIdStr, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "无效的群组ID",
		})
		return
	}

	groupInfo, err := svc.GetGroupInfo(ctx, groupId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "获取群组信息失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "success",
		Data:    groupInfo,
	})
}

// updateGroupMaximum 更新群组最大人数
// @Summary 更新群组最大人数
// @Description 更新指定群组的最大人数限制
// @Tags 群组管理
// @Accept json
// @Produce json
// @Param id path int true "群组ID"
// @Param request body biz.UpdateGroupRequest true "更新请求"
// @Success 200 {object} types.APIResponse
// @Router /api/v1/groupmanage/groups/{id}/maximum [put]
func updateGroupMaximum(c *gin.Context) {
	ctx := context.Background()

	groupIdStr := c.Param("id")
	groupId := new(big.Int)
	_, ok := groupId.SetString(groupIdStr, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "无效的群组ID",
		})
		return
	}
	fmt.Println("groupId:", groupId)
	var req biz.UpdateGroupRequest
	req.GroupId = groupId
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 设置群组ID（从URL参数获取）
	req.GroupId = groupId

	if req.GroupMaximum <= 0 {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "群组最大人数必须大于0",
		})
		return
	}

	err := svc.UpdateGroupMaximum(ctx, groupId, req.GroupMaximum)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "更新群组最大人数失败: " + err.Error(),
		})
		return
	}
	recordAudit(c, "", "update_group_maximum", "group", groupId.String(), strconv.FormatInt(int64(req.GroupMaximum), 10))

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "更新成功",
	})
}

// updateGroupStatus 更新群组状态
// @Summary 更新群组状态
// @Description 更新指定群组的状态
// @Tags 群组管理
// @Accept json
// @Produce json
// @Param id path int true "群组ID"
// @Param request body biz.UpdateGroupRequest true "更新请求"
// @Success 200 {object} types.APIResponse
// @Router /api/v1/groupmanage/groups/{id}/status [put]
func updateGroupStatus(c *gin.Context) {
	ctx := context.Background()

	groupIdStr := c.Param("id")
	groupId := new(big.Int)
	_, ok := groupId.SetString(groupIdStr, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "无效的群组ID",
		})
		return
	}
	if groupId.Cmp(big.NewInt(0)) == 0 {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "无效的群组ID",
		})
		return
	}
	var req biz.UpdateGroupRequest
	req.GroupId = groupId
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 设置群组ID（从URL参数获取）
	req.GroupId = groupId

	if req.GroupStatus < 0 || req.GroupStatus > 2 {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "群组状态值无效，应为0-2",
		})
		return
	}

	err := svc.UpdateGroupStatus(ctx, groupId, req.GroupStatus)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "更新群组状态失败: " + err.Error(),
		})
		return
	}
	recordAudit(c, "", "update_group_status", "group", groupId.String(), groupStatusText(req.GroupStatus))

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "更新成功",
	})
}

// updateGroupName 更新群组名称
// @Summary 更新群组名称
// @Description 更新指定群组的名称
// @Tags 群组管理
// @Accept json
// @Produce json
// @Param id path int true "群组ID"
// @Param request body biz.UpdateGroupRequest true "更新请求"
// @Success 200 {object} types.APIResponse
// @Router /api/v1/groupmanage/groups/{id}/name [put]
func updateGroupName(c *gin.Context) {
	ctx := context.Background()

	groupIdStr := c.Param("id")
	groupId := new(big.Int)
	_, ok := groupId.SetString(groupIdStr, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "无效的群组ID",
		})
		return
	}

	var req biz.UpdateGroupRequest
	req.GroupId = groupId
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 设置群组ID（从URL参数获取）
	req.GroupId = groupId

	if req.GroupName == "" {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "群组名称不能为空",
		})
		return
	}

	err := svc.UpdateGroupName(ctx, groupId, req.GroupName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "更新群组名称失败: " + err.Error(),
		})
		return
	}
	recordAudit(c, "", "update_group_name", "group", groupId.String(), req.GroupName)

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "更新成功",
	})
}

// getGroupStats 获取群组统计信息
// @Summary 获取群组统计信息
// @Description 获取群组总数、活跃群组数、封禁群组数、解散群组数等统计信息
// @Tags 群组管理
// @Accept json
// @Produce json
// @Success 200 {object} types.APIResponse{data=biz.GroupStats}
// @Router /api/v1/groupmanage/groups/stats [get]
func getGroupStats(c *gin.Context) {
	ctx := context.Background()

	stats, err := svc.GetGroupStats(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "获取群组统计信息失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "success",
		Data:    stats,
	})
}

// getTodayGroupCount 获取今日创建的群聊数量
// @Summary 获取今日创建的群聊数量
// @Description 获取今天创建的群聊总数
// @Tags 群组管理
// @Accept json
// @Produce json
// @Success 200 {object} types.APIResponse{data=map[string]int64}
// @Router /api/v1/groupmanage/groups/today-count [get]
func getTodayGroupCount(c *gin.Context) {
	ctx := context.Background()

	count, err := svc.GetTodayGroupCount(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "获取今日群聊数量失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "success",
		Data:    gin.H{"count": count},
	})
}

// getTotalUserCount 获取聊天系统总用户数
// @Summary 获取聊天系统总用户数
// @Description 获取系统中所有不重复用户的总数
// @Tags 群组管理
// @Accept json
// @Produce json
// @Success 200 {object} types.APIResponse{data=map[string]int64}
// @Router /api/v1/groupmanage/users/total-count [get]
func getTotalUserCount(c *gin.Context) {
	ctx := context.Background()

	count, err := svc.GetTotalUserCount(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "获取总用户数失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "success",
		Data:    gin.H{"count": count},
	})
}

// getTodayNewUserCount 获取今日新增用户数
// @Summary 获取今日新增用户数
// @Description 获取今天新加入群聊的用户总数（去重）
// @Tags 群组管理
// @Accept json
// @Produce json
// @Success 200 {object} types.APIResponse{data=map[string]int64}
// @Router /api/v1/groupmanage/users/today-new-count [get]
func getTodayNewUserCount(c *gin.Context) {
	ctx := context.Background()

	count, err := svc.GetTodayNewUserCount(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "获取今日新增用户数失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "success",
		Data:    gin.H{"count": count},
	})
}

// getGroupsByStatus 根据状态查询群组
// @Summary 根据状态查询群组
// @Description 根据群组状态查询群组列表，支持分页
// @Tags 群组管理
// @Accept json
// @Produce json
// @Param status path int true "群组状态" Enums(0,1,2)
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(10)
// @Param keyword query string false "搜索关键词"
// @Success 200 {object} types.APIResponse{data=biz.GroupListResponse}
// @Router /api/v1/groupmanage/groups/status/{status} [get]
func getGroupsByStatus(c *gin.Context) {
	ctx := context.Background()

	// 获取状态参数
	statusStr := c.Param("status")
	status, err := strconv.ParseInt(statusStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "无效的状态参数",
		})
		return
	}

	// 验证状态值
	if status < 0 || status > 2 {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "状态值无效，应为0-2",
		})
		return
	}

	// 解析查询参数
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("pageSize", "10"), 10, 32)
	keyword := c.Query("keyword")

	req := &biz.GroupListRequest{
		Page:     int32(page),
		PageSize: int32(pageSize),
		Keyword:  keyword,
		Status:   int32(status),
	}

	resp, err := svc.GetGroupList(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "根据状态查询群组失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "success",
		Data:    resp,
	})
}

// getWhitelistList 获取白名单列表
// @Summary 获取白名单列表
// @Description 分页获取白名单列表，支持关键词搜索和状态筛选
// @Tags 白名单管理
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(10)
// @Param keyword query string false "搜索关键词"
// @Param status query int false "状态" Enums(0,1)
// @Success 200 {object} types.APIResponse{data=biz.WhitelistListResponse}
// @Router /api/v1/groupmanage/whitelist/list [get]
func getWhitelistList(c *gin.Context) {
	ctx := context.Background()

	// 解析查询参数
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("pageSize", "10"), 10, 32)
	keyword := c.Query("keyword")
	statusStr := c.Query("status")

	var status int32 = -1
	if statusStr != "" {
		if s, err := strconv.ParseInt(statusStr, 10, 32); err == nil {
			status = int32(s)
		}
	}

	req := &biz.WhitelistListRequest{
		Page:     int32(page),
		PageSize: int32(pageSize),
		Keyword:  keyword,
		Status:   status,
	}

	resp, err := svc.GetWhitelistList(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "获取白名单列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "success",
		Data:    resp,
	})
}

// getWhitelistById 根据ID获取白名单信息
// @Summary 获取白名单信息
// @Description 根据ID获取白名单详细信息
// @Tags 白名单管理
// @Accept json
// @Produce json
// @Param id path int true "白名单ID"
// @Success 200 {object} types.APIResponse{data=biz.WhitelistInfo}
// @Router /api/v1/groupmanage/whitelist/{id} [get]
func getWhitelistById(c *gin.Context) {
	ctx := context.Background()

	idStr := c.Param("id")
	id := new(big.Int)
	_, ok := id.SetString(idStr, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "无效的白名单ID",
		})
		return
	}

	whitelist, err := svc.GetWhitelistById(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "获取白名单信息失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "success",
		Data:    whitelist,
	})
}

// getWhitelistByAddress 根据地址获取白名单信息
// @Summary 根据地址获取白名单信息
// @Description 根据用户地址获取白名单详细信息
// @Tags 白名单管理
// @Accept json
// @Produce json
// @Param address query string true "用户地址"
// @Success 200 {object} types.APIResponse{data=biz.WhitelistInfo}
// @Router /api/v1/groupmanage/whitelist/by-address [get]
func getWhitelistByAddress(c *gin.Context) {
	ctx := context.Background()

	address := c.Query("address")
	if address == "" {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "用户地址不能为空",
		})
		return
	}

	whitelist, err := svc.GetWhitelistByAddress(ctx, address)
	if err != nil {
		c.JSON(http.StatusNotFound, types.APIResponse{
			Code:    404,
			Message: "未找到该地址的白名单信息: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "success",
		Data:    whitelist,
	})
}

// addWhitelist 添加白名单
// @Summary 添加白名单
// @Description 添加新的白名单用户
// @Tags 白名单管理
// @Accept json
// @Produce json
// @Param request body biz.AddWhitelistRequest true "添加请求"
// @Success 200 {object} types.APIResponse
// @Router /api/v1/groupmanage/whitelist [post]
func addWhitelist(c *gin.Context) {
	ctx := context.Background()

	var req biz.AddWhitelistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}
	fmt.Printf("req: %+v\n", req)
	err := svc.AddWhitelist(ctx, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "添加白名单失败: " + err.Error(),
		})
		return
	}
	recordAudit(c, "", "add_whitelist", "whitelist", req.Address, req.Remark)

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "添加成功",
	})
}

// updateWhitelist 更新白名单
// @Summary 更新白名单
// @Description 更新白名单信息
// @Tags 白名单管理
// @Accept json
// @Produce json
// @Param id path int true "白名单ID"
// @Param request body biz.UpdateWhitelistRequest true "更新请求"
// @Success 200 {object} types.APIResponse
// @Router /api/v1/groupmanage/whitelist/{id} [put]
func updateWhitelist(c *gin.Context) {
	ctx := context.Background()

	idStr := c.Param("id")
	id := new(big.Int)
	_, ok := id.SetString(idStr, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "无效的白名单ID",
		})
		return
	}

	var req biz.UpdateWhitelistRequest
	req.Id = id
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	req.Id = id
	err := svc.UpdateWhitelist(ctx, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "更新白名单失败: " + err.Error(),
		})
		return
	}
	recordAudit(c, "", "update_whitelist", "whitelist", id.String(), req.Remark)

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "更新成功",
	})
}

// deleteWhitelist 删除白名单
// @Summary 删除白名单
// @Description 删除指定的白名单记录
// @Tags 白名单管理
// @Accept json
// @Produce json
// @Param id path int true "白名单ID"
// @Success 200 {object} types.APIResponse
// @Router /api/v1/groupmanage/whitelist/{id} [delete]
func deleteWhitelist(c *gin.Context) {
	ctx := context.Background()

	idStr := c.Param("id")
	id := new(big.Int)
	_, ok := id.SetString(idStr, 10)
	if !ok {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "无效的白名单ID",
		})
		return
	}

	req := &biz.DeleteWhitelistRequest{Id: id}
	err := svc.DeleteWhitelist(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "删除白名单失败: " + err.Error(),
		})
		return
	}
	recordAudit(c, "", "delete_whitelist", "whitelist", id.String(), "")

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "删除成功",
	})
}

// getAuthNonce 获取登录nonce
// @Router /api/v1/groupmanage/auth/nonce [get]
func getAuthNonce(c *gin.Context) {
	ctx := context.Background()
	address := c.Query("address")
	if address == "" {
		c.JSON(http.StatusBadRequest, types.APIResponse{Code: 400, Message: "address必填"})
		return
	}
	nonce, expireAt, err := svc.GetAuthNonce(ctx, address)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{Code: 500, Message: "获取nonce失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, types.APIResponse{Code: 200, Message: "success", Data: gin.H{"nonce": nonce, "expireAt": expireAt}})
}

type verifyReq struct {
	Address   string `json:"address"`
	Signature string `json:"signature"`
}

// verifyAuth 验签并颁发token
// @Router /api/v1/groupmanage/auth/verify [post]
func verifyAuth(c *gin.Context) {
	ctx := context.Background()
	var req verifyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, types.APIResponse{Code: 400, Message: "请求参数错误: " + err.Error()})
		return
	}
	if req.Address == "" || req.Signature == "" {
		c.JSON(http.StatusBadRequest, types.APIResponse{Code: 400, Message: "address与signature必填"})
		return
	}
	token, exp, err := svc.VerifyAndIssueToken(ctx, req.Address, req.Signature)
	if err != nil {
		recordAudit(c, req.Address, "login_fail", "admin", req.Address, err.Error())
		c.JSON(http.StatusUnauthorized, types.APIResponse{Code: 401, Message: "验签失败: " + err.Error()})
		return
	}
	recordAudit(c, req.Address, "login_ok", "admin", req.Address, "")
	c.JSON(http.StatusOK, types.APIResponse{Code: 200, Message: "success", Data: gin.H{"accessToken": token, "expiresIn": exp}})
}

// AuthMiddleware 简单token校验（占位，可替换为JWT）
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 允许预检请求直接通过
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		fmt.Printf("header: %+v\n", c.Request.Header)
		auth := c.GetHeader("Authorization")
		fmt.Printf("auth: %+v\n", auth)
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, types.APIResponse{Code: 401, Message: "未登录"})
			return
		}
		// JWT 严格校验
		token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, types.APIResponse{Code: 401, Message: "无效token"})
			return
		}
		claims, err := svc.ValidateJWT(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, types.APIResponse{Code: 401, Message: "token校验失败: " + err.Error()})
			return
		}
		// 将地址放入上下文，便于后续操作审计
		c.Set("adminAddress", claims.Address)
		c.Next()
	}
}

// getWhitelistStats 获取白名单统计信息
// @Summary 获取白名单统计信息
// @Description 获取白名单总数、启用数量、禁用数量等统计信息
// @Tags 白名单管理
// @Accept json
// @Produce json
// @Success 200 {object} types.APIResponse{data=biz.WhitelistStats}
// @Router /api/v1/groupmanage/whitelist/stats [get]
func getWhitelistStats(c *gin.Context) {
	ctx := context.Background()

	stats, err := svc.GetWhitelistStats(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "获取白名单统计信息失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "success",
		Data:    stats,
	})
}

// getGroupsByOwner 根据创建者地址获取群组列表
// @Summary 根据创建者地址获取群组列表
// @Description 分页获取指定地址创建的群组列表
// @Tags 群组管理
// @Accept json
// @Produce json
// @Param ownerId query string true "创建者地址"
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(10)
// @Success 200 {object} types.APIResponse{data=biz.GroupListResponse}
// @Router /api/v1/groupmanage/groups/by-owner [get]
func getGroupsByOwner(c *gin.Context) {
	ctx := context.Background()

	ownerId := c.Query("ownerId")
	if ownerId == "" {
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "创建者地址不能为空",
		})
		return
	}

	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("pageSize", "10"), 10, 32)

	resp, err := svc.GetGroupsByOwner(ctx, ownerId, int32(page), int32(pageSize))
	if err != nil {
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "获取群组列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "success",
		Data:    resp,
	})
}

// getGroupMembers 获取群成员列表
// @Summary 获取群成员列表
// @Description 根据群组ID分页获取群成员列表
// @Tags 群组管理
// @Accept json
// @Produce json
// @Param id path int true "群组ID"
// @Param page query int false "页码，从 1 开始" default(1)
// @Param pageSize query int false "每页条数，最大 100" default(10)
// @Success 200 {object} types.APIResponse
// @Router /api/v1/groupmanage/groups/{id}/members [get]
func getGroupMembers(c *gin.Context) {
	ctx := context.Background()

	groupIdStr := c.Param("id")
	groupId := new(big.Int)
	_, ok := groupId.SetString(groupIdStr, 10)
	if !ok {
		log.Error().Str("groupId", groupIdStr).Msg("获取群成员列表失败: 无效的群组ID")
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "无效的群组ID",
		})
		return
	}

	page, pageSize, err := memberPageQuery(c)
	if err != nil {
		log.Error().Err(err).Str("groupId", groupIdStr).Msg("获取群成员列表失败: 分页参数无效")
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	members, total, err := svc.GetGroupMembers(ctx, groupId, page, pageSize)
	if err != nil {
		log.Error().Err(err).Str("groupId", groupIdStr).Int32("page", page).Int32("pageSize", pageSize).Msg("获取群成员列表失败")
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "获取群成员列表失败: " + err.Error(),
		})
		return
	}
	if members == nil {
		members = []*biz.GroupMember{}
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "success",
		Data: gin.H{
			"list":     members,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}

func memberPageQuery(c *gin.Context) (int32, int32, error) {
	page := int32(1)
	if raw := c.Query("page"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || n < 1 {
			return 0, 0, fmt.Errorf("无效的页码")
		}
		page = int32(n)
	}
	pageSize := int32(10)
	if raw := c.Query("pageSize"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || n < 1 || n > 100 {
			return 0, 0, fmt.Errorf("无效的每页条数")
		}
		pageSize = int32(n)
	}
	return page, pageSize, nil
}

// getUserLoginHistory 根据用户地址查询登录历史及账号绑定信息
// @Summary 查询用户登录历史
// @Description 根据链上地址查询登录记录，并关联手机号、微信号、小红书号等绑定信息
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param address query string true "用户链上地址"
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Param startTime query int64 false "开始时间（毫秒时间戳）"
// @Param endTime query int64 false "结束时间（毫秒时间戳）"
// @Success 200 {object} types.APIResponse{data=biz.UserLoginWithBinding}
// @Router /api/v1/groupmanage/users/login-history [get]
func getUserLoginHistory(c *gin.Context) {
	ctx := context.Background()

	address := strings.TrimSpace(c.Query("address"))
	if address == "" {
		log.Error().Msg("查询用户登录历史失败: 用户地址为空")
		c.JSON(http.StatusBadRequest, types.APIResponse{
			Code:    400,
			Message: "用户地址不能为空",
		})
		return
	}

	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("pageSize", "20"), 10, 32)
	startTime, _ := strconv.ParseInt(c.Query("startTime"), 10, 64)
	endTime, _ := strconv.ParseInt(c.Query("endTime"), 10, 64)

	req := &biz.UserLoginHistoryRequest{
		Address:   address,
		Page:      int32(page),
		PageSize:  int32(pageSize),
		StartTime: startTime,
		EndTime:   endTime,
	}

	result, err := svc.GetUserLoginHistory(ctx, req)
	if err != nil {
		log.Error().Err(err).Str("address", address).Int32("page", req.Page).Int32("pageSize", req.PageSize).Int64("startTime", startTime).Int64("endTime", endTime).Msg("查询用户登录历史失败")
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "查询用户登录历史失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "success",
		Data:    result,
	})
}

// getAllLoginLogs 分页查询全站用户登录日志（可选多个 uid 并行筛选：?uid=a&uid=b）
// @Summary 全站登录日志列表
// @Description 分页返回 dtalk_login_log 全表记录，支持时间范围与多个 uid（OR）
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Param startTime query int64 false "开始时间（毫秒时间戳）"
// @Param endTime query int64 false "结束时间（毫秒时间戳）"
// @Param uid query []string false "用户地址，可重复传入多个实现并行筛选"
// @Success 200 {object} types.APIResponse{data=biz.AllLoginLogsResponse}
// @Router /api/v1/groupmanage/users/login-logs [get]
func getAllLoginLogs(c *gin.Context) {
	ctx := context.Background()

	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("pageSize", "20"), 10, 32)
	startTime, _ := strconv.ParseInt(c.Query("startTime"), 10, 64)
	endTime, _ := strconv.ParseInt(c.Query("endTime"), 10, 64)
	uids := c.QueryArray("uid")

	req := &biz.AllLoginLogsRequest{
		Page:      int32(page),
		PageSize:  int32(pageSize),
		StartTime: startTime,
		EndTime:   endTime,
		Uid:       uids,
	}

	result, err := svc.GetAllLoginLogs(ctx, req)
	if err != nil {
		fmt.Printf("[HTTP Handler] getAllLoginLogs 失败: page=%d pageSize=%d uidCount=%d startTime=%d endTime=%d err=%v\n",
			req.Page, req.PageSize, len(req.Uid), req.StartTime, req.EndTime, err)
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "查询全站登录日志失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, types.APIResponse{
		Code:    200,
		Message: "success",
		Data:    result,
	})
}

// listDailyActive 查询每日活跃入库记录（支持 startDate/endDate 与多 date）
func listDailyActive(c *gin.Context) {
	ctx := context.Background()
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("pageSize", "30"), 10, 32)
	req := &biz.DailyActiveListRequest{
		StartDate: c.Query("startDate"),
		EndDate:   c.Query("endDate"),
		Dates:     c.QueryArray("date"),
		Page:      int32(page),
		PageSize:  int32(pageSize),
	}
	result, err := svc.ListDailyActive(ctx, req)
	if err != nil {
		fmt.Printf("[HTTP Handler] listDailyActive failed: startDate=%s endDate=%s dates=%v page=%d err=%v\n",
			req.StartDate, req.EndDate, req.Dates, req.Page, err)
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "查询每日活跃失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, types.APIResponse{Code: 200, Message: "success", Data: result})
}

func listActiveAddresses(c *gin.Context) {
	day, ok := parseStatDateQuery(c)
	if !ok {
		return
	}
	page, pageSize := detailPageQuery(c)
	result, err := svc.ListActiveAddresses(c.Request.Context(), day, page, pageSize)
	if err != nil {
		log.Error().Err(err).Str("date", day.Format("2006-01-02")).Int32("page", page).Int32("pageSize", pageSize).Msg("查询每日活跃地址失败")
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "查询每日活跃地址失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, types.APIResponse{Code: 200, Message: "success", Data: result})
}

func listActiveGroups(c *gin.Context) {
	day, ok := parseStatDateQuery(c)
	if !ok {
		return
	}
	page, pageSize := detailPageQuery(c)
	result, err := svc.ListActiveGroups(c.Request.Context(), day, page, pageSize)
	if err != nil {
		log.Error().Err(err).Str("date", day.Format("2006-01-02")).Int32("page", page).Int32("pageSize", pageSize).Msg("查询每日活跃群聊失败")
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "查询每日活跃群聊失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, types.APIResponse{Code: 200, Message: "success", Data: result})
}

func parseStatDateQuery(c *gin.Context) (time.Time, bool) {
	dateStr := strings.TrimSpace(c.Query("date"))
	if dateStr == "" {
		return time.Now(), true
	}
	day, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
	if err != nil {
		log.Error().Str("date", dateStr).Msg("每日活跃日期格式无效")
		c.JSON(http.StatusBadRequest, types.APIResponse{Code: 400, Message: "date 格式应为 YYYY-MM-DD"})
		return time.Time{}, false
	}
	return day, true
}

func detailPageQuery(c *gin.Context) (int32, int32) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("pageSize", "50"), 10, 32)
	return int32(page), int32(pageSize)
}

// snapshotDailyActive 手动触发指定日（默认今天）活跃快照入库
func snapshotDailyActive(c *gin.Context) {
	ctx := context.Background()
	dateStr := c.Query("date")
	day := time.Now()
	if dateStr != "" {
		parsed, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, types.APIResponse{Code: 400, Message: "date 格式应为 YYYY-MM-DD"})
			return
		}
		day = parsed
	}
	rec, err := svc.SnapshotDailyActive(ctx, day)
	if err != nil {
		fmt.Printf("[HTTP Handler] snapshotDailyActive failed: date=%s err=%v\n", day.Format("2006-01-02"), err)
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "快照入库失败: " + err.Error(),
		})
		return
	}
	recordAudit(c, "", "snapshot_daily_active", "metric", day.Format("2006-01-02"), "")
	c.JSON(http.StatusOK, types.APIResponse{Code: 200, Message: "success", Data: rec})
}

func logoutAuth(c *gin.Context) {
	recordAudit(c, "", "logout", "admin", operatorAddress(c), "")
	c.JSON(http.StatusOK, types.APIResponse{Code: 200, Message: "success"})
}

// listAudits 分页查询操作留痕。action、targetType 可重复传入，多个值是或关系，和其他条件是并且关系。
func listAudits(c *gin.Context) {
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 32)
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("pageSize", "20"), 10, 32)
	result, err := svc.ListAudit(c.Request.Context(),
		c.QueryArray("action"),
		c.QueryArray("targetType"),
		c.Query("targetId"),
		c.Query("operator"),
		int32(page),
		int32(pageSize),
	)
	if err != nil {
		log.Error().Err(err).
			Strs("action", c.QueryArray("action")).
			Strs("targetType", c.QueryArray("targetType")).
			Str("targetId", c.Query("targetId")).
			Str("operator", c.Query("operator")).
			Msg("查询操作留痕失败")
		c.JSON(http.StatusInternalServerError, types.APIResponse{
			Code:    500,
			Message: "查询操作留痕失败: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, types.APIResponse{Code: 200, Message: "success", Data: result})
}

func recordAudit(c *gin.Context, operator, action, targetType, targetID, extra string) {
	if operator == "" {
		operator = operatorAddress(c)
	}
	svc.RecordAudit(c.Request.Context(), &biz.AdminAudit{
		Operator:   operator,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Extra:      extra,
		ClientIP:   c.ClientIP(),
	})
}

func operatorAddress(c *gin.Context) string {
	v, ok := c.Get("adminAddress")
	if !ok {
		return ""
	}
	addr, _ := v.(string)
	return addr
}

func groupStatusText(status int32) string {
	switch status {
	case 0:
		return "正常"
	case 1:
		return "封禁"
	case 2:
		return "解散"
	default:
		return strconv.FormatInt(int64(status), 10)
	}
}
