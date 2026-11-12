package logic

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc/metadata"
)

// AddressHelper 地址获取辅助工具
type AddressHelper struct{}

// NewAddressHelper 创建地址获取辅助工具
func NewAddressHelper() *AddressHelper {
	return &AddressHelper{}
}

// GetAddressFromContext 从 context 中获取用户地址
func (h *AddressHelper) GetAddressFromContext(ctx context.Context) (string, error) {
	// 方法1: 从 gRPC metadata 中获取
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if addresses := md.Get("user-address"); len(addresses) > 0 {
			return addresses[0], nil
		}
		if addresses := md.Get("x-user-address"); len(addresses) > 0 {
			return addresses[0], nil
		}
	}

	// 方法2: 从 context 的 value 中获取
	if addr, ok := ctx.Value("user-address").(string); ok && addr != "" {
		return addr, nil
	}

	// 方法3: 从 context 的 value 中获取（使用自定义 key）
	if addr, ok := ctx.Value("address").(string); ok && addr != "" {
		return addr, nil
	}

	return "", fmt.Errorf("无法从 context 中获取用户地址")
}

// GetAddressFromSignature 从签名中解析地址
// 这里需要根据实际的签名算法来实现
func (h *AddressHelper) GetAddressFromSignature(signature string) (string, error) {
	// 假设签名格式为: <signature>#<message>#<publicKey>
	// 需要从 publicKey 中解析出地址
	parts := strings.Split(signature, "#")
	if len(parts) != 3 {
		return "", fmt.Errorf("签名格式错误")
	}

	// 这里需要根据实际的公钥到地址的转换逻辑来实现
	// 例如使用以太坊的地址生成算法
	publicKey := parts[2]
	address := h.publicKeyToAddress(publicKey)

	return address, nil
}

// publicKeyToAddress 将公钥转换为地址
// 这里需要根据实际的区块链地址生成算法来实现
func (h *AddressHelper) publicKeyToAddress(publicKey string) string {
	// 临时实现：直接返回公钥
	// 实际使用时需要根据具体的区块链地址生成算法来实现
	return publicKey
}

// ValidateAddress 验证地址格式
func (h *AddressHelper) ValidateAddress(address string) error {
	if address == "" {
		return fmt.Errorf("地址不能为空")
	}

	// 这里可以添加地址格式验证逻辑
	// 例如验证以太坊地址格式、长度等

	if len(address) < 20 {
		return fmt.Errorf("地址长度不足")
	}

	return nil
}

// SetAddressToContext 将地址设置到 context 中
func (h *AddressHelper) SetAddressToContext(ctx context.Context, address string) context.Context {
	return context.WithValue(ctx, "user-address", address)
}
