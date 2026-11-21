package handler

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	dtalkauth "github.com/web3chat/txchat/dtalk/pkg/auth"
	"github.com/web3chat/txchat/dtalk/service/shardkey/config"
	"github.com/web3chat/txchat/dtalk/service/shardkey/internal/service"
	"github.com/web3chat/txchat/dtalk/service/shardkey/internal/store"
)

// APIResponse 统一响应结构：result=0 成功，-1 失败。
type APIResponse struct {
	Result  int         `json:"result"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

const (
	ResultOK    = 0
	ResultError = -1
)

func newResponse(result int, message string, data interface{}) APIResponse {
	if data == nil {
		data = gin.H{}
	}
	return APIResponse{Result: result, Message: message, Data: data}
}

func successResponse(message string, data interface{}) APIResponse {
	return newResponse(ResultOK, message, data)
}

func errorResponse(message string, data interface{}) APIResponse {
	return newResponse(ResultError, message, data)
}

// RegisterRoutes 注册路由。
func RegisterRoutes(r *gin.Engine, cfg *config.Config, smsService *service.SMSService, shardStore *store.ShardStore, bindingStore *store.BindingStore) {
	r.GET("/api/v1/health", healthHandler())
	r.HEAD("/api/v1/health", healthHandler())

	api := r.Group("/api/v1", apiKeyAuthMiddleware(cfg.APIKeys))
	{
		api.POST("/send-code", sendCodeHandler(smsService))
		api.POST("/verify-code", verifyCodeHandler(smsService))
		api.POST("/store-shard", storeShardHandler(smsService, shardStore, bindingStore))
		api.POST("/get-shard", getShardHandler(smsService, shardStore))
		api.POST("/get-shard-by-sms", getShardBySMSHandler(smsService, shardStore))
		api.POST("/reset-shard", resetShardHandler(smsService, shardStore))
		api.POST("/check-shard-exists", checkShardExistsHandler(shardStore, bindingStore))
		api.POST("/delete-shard", deleteShardHandler(smsService, shardStore))

		// 账号绑定相关：必须带 FZM-SIGNATURE，并校验聊天地址归属
		api.POST("/bind-account", bindAccountHandler(bindingStore, shardStore))
		api.POST("/query-chataddress", queryChatAddressHandler(bindingStore))
		api.POST("/query-accounts", queryAccountsByChatAddressHandler(bindingStore))
	}
}

func apiKeyAuthMiddleware(apiKeys map[string]string) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID := strings.TrimSpace(c.GetHeader("X-App-Id"))
		token := strings.TrimSpace(c.GetHeader("X-App-Token"))

		if appID == "" || token == "" {
			logrus.WithFields(logrus.Fields{
				"path": c.Request.URL.Path,
				"ip":   c.ClientIP(),
			}).Warn("missing API auth headers")
			c.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse("missing API auth headers", nil))
			return
		}

		expected, ok := apiKeys[appID]
		if !ok || expected == "" || expected != token {
			// 不打印 expected/token，避免密钥进日志
			logrus.WithFields(logrus.Fields{
				"path":   c.Request.URL.Path,
				"app_id": appID,
				"ip":     c.ClientIP(),
			}).Warn("invalid API credentials")
			c.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse("unauthorized client", nil))
			return
		}

		c.Set("app_id", appID)
		c.Next()
	}
}

// verifyAddressSignature 用 FZM-SIGNATURE 校验请求者确实持有 addr 对应的私钥。
func verifyAddressSignature(c *gin.Context, addr string) error {
	sig := strings.TrimSpace(c.GetHeader("FZM-SIGNATURE"))
	if sig == "" {
		return fmt.Errorf("missing signature")
	}
	server := dtalkauth.NewDefaultApiAuthenticator()
	if strings.HasPrefix(strings.ToLower(addr), "0x") {
		uid, err := server.Auth4EthAcc(sig)
		if err != nil {
			return err
		}
		if !strings.EqualFold(uid, addr) {
			return fmt.Errorf("signature address mismatch")
		}
		return nil
	}
	uid, err := server.Auth(sig)
	if err != nil {
		return err
	}
	if uid != addr {
		return fmt.Errorf("signature address mismatch")
	}
	return nil
}

type SendCodeRequest struct {
	Phone   string `json:"phone" binding:"required"`
	Purpose string `json:"purpose" binding:"required,oneof=backup reset get delete"`
}

func sendCodeHandler(smsService *service.SMSService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req SendCodeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid request format", nil))
			return
		}
		if _, ok := service.PurposeName(req.Purpose); !ok {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid purpose, must be one of: backup, reset, get, delete", nil))
			return
		}
		if err := smsService.SendVerificationCode(req.Phone, req.Purpose); err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse(err.Error(), nil))
			return
		}
		c.JSON(http.StatusOK, successResponse("Verification code sent successfully", gin.H{
			"phone":   req.Phone,
			"purpose": req.Purpose,
		}))
	}
}

type VerifyCodeRequest struct {
	Phone string `json:"phone" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

func verifyCodeHandler(smsService *service.SMSService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req VerifyCodeRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid request format", nil))
			return
		}
		valid, err := smsService.VerifyCode(req.Phone, req.Code)
		if err != nil {
			c.JSON(http.StatusBadRequest, errorResponse(err.Error(), gin.H{"valid": false}))
			return
		}
		c.JSON(http.StatusOK, successResponse("Verification successful", gin.H{
			"phone": req.Phone,
			"valid": valid,
		}))
	}
}

type StoreShardRequest struct {
	Phone       string `json:"phone" binding:"required"`
	Code        string `json:"code" binding:"required"`
	ChatAddress string `json:"chataddress" binding:"required"`
	ShardKey    string `json:"shard_key" binding:"required"`
}

func storeShardHandler(smsService *service.SMSService, shardStore *store.ShardStore, bindingStore *store.BindingStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req StoreShardRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid request format", nil))
			return
		}
		if !isValidAddress(req.ChatAddress) {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid chataddress format, must start with 0x or 1x", nil))
			return
		}
		if !validShardKey(req.ShardKey) {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid shard_key", nil))
			return
		}

		if existing, exists := shardStore.GetByPhone(req.Phone); exists {
			c.JSON(http.StatusConflict, errorResponse(
				fmt.Sprintf("Shard key already exists for this phone number. Please use /reset-shard to update it. Current chataddress: %s", existing.ChatAddress),
				gin.H{"phone": existing.Phone, "chataddress": existing.ChatAddress}))
			return
		}
		// 手机号已绑定到其他地址时不允许直接覆盖，避免账号分叉
		if boundAddr, bound := bindingStore.GetChatAddressByPhone(req.Phone); bound && !strings.EqualFold(boundAddr, req.ChatAddress) {
			c.JSON(http.StatusConflict, errorResponse(
				"Phone number is already bound to another account. Please log in with the bound account or reset it first.",
				gin.H{"phone": req.Phone, "chataddress": boundAddr}))
			return
		}

		valid, err := smsService.VerifyCodeWithPurpose(req.Phone, req.Code, "backup")
		if err != nil || !valid {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid verification code or purpose mismatch. Please use verification code with purpose 'backup'", gin.H{"phone": req.Phone}))
			return
		}

		if err := shardStore.Save(req.Phone, req.ChatAddress, req.ShardKey); err != nil {
			logrus.WithError(err).WithField("phone", maskPhone(req.Phone)).Error("save shard key failed")
			c.JSON(http.StatusInternalServerError, errorResponse("Failed to save shard key", gin.H{"phone": req.Phone}))
			return
		}
		c.JSON(http.StatusOK, successResponse("Shard key stored successfully", gin.H{
			"phone":       req.Phone,
			"chataddress": req.ChatAddress,
		}))
	}
}

func isValidAddress(addr string) bool {
	return (strings.HasPrefix(addr, "0x") || strings.HasPrefix(addr, "1")) && len(addr) <= 128
}

func validShardKey(key string) bool {
	return key != "" && len(key) <= 512
}

func maskPhone(phone string) string {
	if len(phone) != 11 {
		return phone
	}
	return phone[:3] + "****" + phone[7:]
}

type GetShardRequest struct {
	Phone string `json:"phone" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

func getShardHandler(smsService *service.SMSService, shardStore *store.ShardStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req GetShardRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid request format", nil))
			return
		}
		valid, err := smsService.VerifyCodeWithPurpose(req.Phone, req.Code, "get")
		if err != nil || !valid {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid verification code or purpose mismatch. Please use verification code with purpose 'get'", gin.H{"phone": req.Phone}))
			return
		}
		rec, ok := shardStore.GetByPhone(req.Phone)
		if !ok {
			c.JSON(http.StatusNotFound, errorResponse("Shard key not found", gin.H{"phone": req.Phone}))
			return
		}
		c.JSON(http.StatusOK, successResponse("Shard key fetched successfully", gin.H{
			"phone":     rec.Phone,
			"shard_key": rec.ShardKey,
		}))
	}
}

type GetShardBySMSRequest struct {
	Phone string `json:"phone" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

func getShardBySMSHandler(smsService *service.SMSService, shardStore *store.ShardStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req GetShardBySMSRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid request format", nil))
			return
		}
		valid, err := smsService.VerifyCodeWithPurpose(req.Phone, req.Code, "get")
		if err != nil || !valid {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid verification code or purpose mismatch. Please use verification code with purpose 'get'", gin.H{"phone": req.Phone}))
			return
		}
		rec, ok := shardStore.GetByPhone(req.Phone)
		if !ok {
			c.JSON(http.StatusNotFound, errorResponse("Shard key not found", gin.H{"phone": req.Phone}))
			return
		}
		smsContent := fmt.Sprintf("密钥分片：\n\n%s\n\n请妥善保管，勿泄露。", rec.ShardKey)
		if err := smsService.SendCustomSMS(req.Phone, smsContent); err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse("Failed to send SMS", gin.H{"phone": req.Phone}))
			return
		}
		c.JSON(http.StatusOK, successResponse("Shard key has been sent to your phone via SMS", gin.H{"phone": req.Phone}))
	}
}

type ResetShardRequest struct {
	Phone       string `json:"phone" binding:"required"`
	Code        string `json:"code" binding:"required"`
	ChatAddress string `json:"chataddress" binding:"required"`
	ShardKey    string `json:"shard_key" binding:"required"`
}

func resetShardHandler(smsService *service.SMSService, shardStore *store.ShardStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ResetShardRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid request format", nil))
			return
		}
		if !isValidAddress(req.ChatAddress) {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid chataddress format, must start with 0x or 1x", nil))
			return
		}
		if !validShardKey(req.ShardKey) {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid shard_key", nil))
			return
		}
		valid, err := smsService.VerifyCodeWithPurpose(req.Phone, req.Code, "reset")
		if err != nil || !valid {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid verification code or purpose mismatch. Please use verification code with purpose 'reset'", gin.H{"phone": req.Phone}))
			return
		}
		if _, exists := shardStore.GetByPhone(req.Phone); !exists {
			c.JSON(http.StatusNotFound, errorResponse("Shard key not found, please use store-shard to create first", gin.H{"phone": req.Phone}))
			return
		}
		if err := shardStore.Reset(req.Phone, req.ChatAddress, req.ShardKey); err != nil {
			logrus.WithError(err).WithField("phone", maskPhone(req.Phone)).Error("reset shard key failed")
			c.JSON(http.StatusInternalServerError, errorResponse("Failed to reset shard key", gin.H{"phone": req.Phone}))
			return
		}
		c.JSON(http.StatusOK, successResponse("Shard key reset successfully", gin.H{"phone": req.Phone, "is_updated": true}))
	}
}

type DeleteShardRequest struct {
	Phone string `json:"phone" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

func deleteShardHandler(smsService *service.SMSService, shardStore *store.ShardStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req DeleteShardRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid request format", nil))
			return
		}
		valid, err := smsService.VerifyCodeWithPurpose(req.Phone, req.Code, "delete")
		if err != nil || !valid {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid verification code or purpose mismatch. Please use verification code with purpose 'delete'", gin.H{"phone": req.Phone}))
			return
		}
		rec, exists := shardStore.GetByPhone(req.Phone)
		if !exists {
			c.JSON(http.StatusNotFound, errorResponse("Shard key not found", gin.H{"phone": req.Phone}))
			return
		}
		if err := shardStore.Delete(req.Phone); err != nil {
			c.JSON(http.StatusInternalServerError, errorResponse("Failed to delete shard key", gin.H{"phone": req.Phone}))
			return
		}
		c.JSON(http.StatusOK, successResponse("Shard key deleted successfully", gin.H{
			"phone":       req.Phone,
			"chataddress": rec.ChatAddress,
		}))
	}
}

type CheckShardExistsRequest struct {
	Phone string `json:"phone" binding:"required"`
}

// checkShardExistsHandler 查询手机号是否已有分片。
// 如果手机号已在 user_bindings 绑定、但没有分片，返回 exists=true + shard_missing=true，
// 避免 App 走“新用户”分支生成新地址，导致旧账号被孤儿化。
func checkShardExistsHandler(shardStore *store.ShardStore, bindingStore *store.BindingStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CheckShardExistsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid request format", nil))
			return
		}
		rec, exists := shardStore.GetByPhone(req.Phone)
		if exists {
			c.JSON(http.StatusOK, successResponse("Shard key exists", gin.H{
				"phone":       rec.Phone,
				"exists":      true,
				"chataddress": rec.ChatAddress,
				"is_updated":  rec.IsUpdated,
				"version":     rec.Version,
			}))
			return
		}
		if boundAddr, bound := bindingStore.GetChatAddressByPhone(req.Phone); bound {
			c.JSON(http.StatusOK, successResponse("Phone is bound but shard is missing", gin.H{
				"phone":         req.Phone,
				"exists":        true,
				"shard_missing": true,
				"chataddress":   boundAddr,
			}))
			return
		}
		c.JSON(http.StatusOK, errorResponse("Shard key not found", gin.H{
			"phone":       req.Phone,
			"exists":      false,
			"chataddress": "",
		}))
	}
}

func healthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, successResponse("Service is running", gin.H{
			"status":  "healthy",
			"message": "Service is running",
			"version": "1.0.0",
		}))
	}
}

// ============================================
// 账号绑定相关接口
// ============================================

type BindAccountRequest struct {
	Phone         string `json:"phone"`
	WechatID      string `json:"wechat_id"`
	XiaohongshuID string `json:"xiaohongshu_id"`
	DouyinID      string `json:"douyin_id"`
	ChatAddress   string `json:"chataddress" binding:"required"`
}

// bindAccountHandler 将手机号/微信号/小红书/抖音绑定到聊天地址。
// 必须带 FZM-SIGNATURE；绑定手机号时还要求该手机号当前分片属于同一地址，
// 证明调用者刚刚通过短信验证过该手机号。
func bindAccountHandler(bindingStore *store.BindingStore, shardStore *store.ShardStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req BindAccountRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid request format", nil))
			return
		}
		if !isValidAddress(req.ChatAddress) {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid chataddress format, must start with 0x or 1x", nil))
			return
		}
		if req.Phone == "" && req.WechatID == "" && req.XiaohongshuID == "" && req.DouyinID == "" {
			c.JSON(http.StatusBadRequest, errorResponse("至少需要提供一个账号标识（手机号、微信号、小红书账号或抖音号）", nil))
			return
		}
		if err := verifyAddressSignature(c, req.ChatAddress); err != nil {
			c.JSON(http.StatusUnauthorized, errorResponse("invalid or missing FZM-SIGNATURE", nil))
			return
		}
		if req.Phone != "" {
			// 手机号已绑定到其他地址时，必须有同一地址的当前分片（说明刚完成短信验证/重置），
			// 否则禁止把别人的手机号抢绑到本地址。
			if boundAddr, bound := bindingStore.GetChatAddressByPhone(req.Phone); bound && !strings.EqualFold(boundAddr, req.ChatAddress) {
				rec, ok := shardStore.GetByPhone(req.Phone)
				if !ok || !strings.EqualFold(rec.ChatAddress, req.ChatAddress) {
					c.JSON(http.StatusForbidden, errorResponse(
						"phone is already bound to another chat address",
						gin.H{"phone": req.Phone, "chataddress": boundAddr}))
					return
				}
			}
		}
		if err := bindingStore.BindAccount(req.Phone, req.WechatID, req.XiaohongshuID, req.DouyinID, req.ChatAddress); err != nil {
			logrus.WithError(err).WithField("chataddress", req.ChatAddress).Error("bind account failed")
			c.JSON(http.StatusInternalServerError, errorResponse(err.Error(), gin.H{"chataddress": req.ChatAddress}))
			return
		}
		c.JSON(http.StatusOK, successResponse("Account bound successfully", gin.H{
			"phone":          req.Phone,
			"wechat_id":      req.WechatID,
			"xiaohongshu_id": req.XiaohongshuID,
			"douyin_id":      req.DouyinID,
			"chataddress":    req.ChatAddress,
		}))
	}
}

type QueryChatAddressRequest struct {
	Keyword string `json:"keyword" binding:"required"`
}

func resolveChatAddressByKeyword(bindingStore *store.BindingStore, keyword string) (chatAddress, matchedField string, ok bool) {
	var wg sync.WaitGroup
	var phoneAddr, wechatAddr, xhsAddr, douyinAddr string
	var phoneOk, wechatOk, xhsOk, douyinOk bool
	wg.Add(4)
	go func() { defer wg.Done(); phoneAddr, phoneOk = bindingStore.GetChatAddressByPhone(keyword) }()
	go func() { defer wg.Done(); wechatAddr, wechatOk = bindingStore.GetChatAddressByWechatID(keyword) }()
	go func() { defer wg.Done(); xhsAddr, xhsOk = bindingStore.GetChatAddressByXiaohongshuID(keyword) }()
	go func() { defer wg.Done(); douyinAddr, douyinOk = bindingStore.GetChatAddressByDouyinID(keyword) }()
	wg.Wait()
	if phoneOk {
		return phoneAddr, "phone", true
	}
	if wechatOk {
		return wechatAddr, "wechat_id", true
	}
	if xhsOk {
		return xhsAddr, "xiaohongshu_id", true
	}
	if douyinOk {
		return douyinAddr, "douyin_id", true
	}
	return "", "", false
}

func queryChatAddressHandler(bindingStore *store.BindingStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req QueryChatAddressRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid request format", nil))
			return
		}
		keyword := strings.TrimSpace(req.Keyword)
		if keyword == "" {
			c.JSON(http.StatusBadRequest, errorResponse("keyword 不能为空", nil))
			return
		}
		chatAddress, matchedField, found := resolveChatAddressByKeyword(bindingStore, keyword)
		if !found {
			c.JSON(http.StatusNotFound, errorResponse("Chat address not found", gin.H{"keyword": keyword}))
			return
		}
		c.JSON(http.StatusOK, successResponse("Chat address found", gin.H{
			"chataddress":   chatAddress,
			"keyword":       keyword,
			"matched_field": matchedField,
		}))
	}
}

type QueryAccountsRequest struct {
	ChatAddress string `json:"chataddress" binding:"required"`
}

func queryAccountsByChatAddressHandler(bindingStore *store.BindingStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req QueryAccountsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid request format", nil))
			return
		}
		if !isValidAddress(req.ChatAddress) {
			c.JSON(http.StatusBadRequest, errorResponse("Invalid chataddress format, must start with 0x or 1x", nil))
			return
		}
		if err := verifyAddressSignature(c, req.ChatAddress); err != nil {
			c.JSON(http.StatusUnauthorized, errorResponse("invalid or missing FZM-SIGNATURE", nil))
			return
		}
		rec, found := bindingStore.GetAccountsByChatAddress(req.ChatAddress)
		if !found {
			c.JSON(http.StatusNotFound, errorResponse("Accounts not found for this chat address", gin.H{"chataddress": req.ChatAddress}))
			return
		}
		c.JSON(http.StatusOK, successResponse("Accounts found", gin.H{
			"chataddress":    rec.ChatAddress,
			"phone":          rec.Phone,
			"wechat_id":      rec.WechatID,
			"xiaohongshu_id": rec.XiaohongshuID,
			"douyin_id":      rec.DouyinID,
		}))
	}
}
