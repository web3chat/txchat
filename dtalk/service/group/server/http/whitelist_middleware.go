package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/web3chat/txchat/dtalk/service/group/logic"
)

// WhitelistMiddleware 白名单中间件
// 这个中间件用于在 HTTP 层获取用户地址，并将其传递给 gRPC 层
type WhitelistMiddleware struct {
	addressHelper *logic.AddressHelper
}

// NewWhitelistMiddleware 创建白名单中间件
func NewWhitelistMiddleware() *WhitelistMiddleware {
	return &WhitelistMiddleware{
		addressHelper: logic.NewAddressHelper(),
	}
}

// Middleware 中间件函数
func (m *WhitelistMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 从请求头中获取地址
		address := m.getAddressFromRequest(r)

		// 如果无法从请求头获取，尝试从签名中解析
		if address == "" {
			signature := r.Header.Get("FZM-SIGNATURE")
			if signature != "" {
				var err error
				address, err = m.addressHelper.GetAddressFromSignature(signature)
				if err != nil {
					http.Error(w, "无法解析用户地址", http.StatusBadRequest)
					return
				}
			}
		}

		// 验证地址格式
		if address != "" {
			if err := m.addressHelper.ValidateAddress(address); err != nil {
				http.Error(w, "地址格式无效", http.StatusBadRequest)
				return
			}

			// 将地址添加到 context 中
			ctx := m.addressHelper.SetAddressToContext(r.Context(), address)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

// getAddressFromRequest 从 HTTP 请求中获取地址
func (m *WhitelistMiddleware) getAddressFromRequest(r *http.Request) string {
	// 方法1: 从自定义请求头中获取
	if address := r.Header.Get("X-User-Address"); address != "" {
		return address
	}

	// 方法2: 从 Authorization 头中获取
	if auth := r.Header.Get("Authorization"); auth != "" {
		// 这里可以解析 JWT token 或其他认证信息
		// 临时实现：直接返回
		return ""
	}

	// 方法3: 从请求参数中获取
	if address := r.URL.Query().Get("address"); address != "" {
		return address
	}

	// 方法4: 从请求体中获取（如果是 POST 请求）
	if r.Method == "POST" && strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		// 这里可以解析 JSON 请求体
		// 临时实现：直接返回
		return ""
	}

	return ""
}

// GetAddressFromContext 从 context 中获取地址（供 gRPC 层使用）
func (m *WhitelistMiddleware) GetAddressFromContext(ctx context.Context) (string, error) {
	return m.addressHelper.GetAddressFromContext(ctx)
}
