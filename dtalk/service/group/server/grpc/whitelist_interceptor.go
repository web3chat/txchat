package grpc

import (
	"context"
	"strings"

	"github.com/web3chat/txchat/dtalk/service/group/logic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// WhitelistInterceptor gRPC 白名单拦截器
// 这个拦截器用于在 gRPC 层获取用户地址，并将其传递给业务逻辑层
func WhitelistInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 只对创建群组的接口进行白名单检查
		if strings.Contains(info.FullMethod, "CreateGroup") {
			addressHelper := logic.NewAddressHelper()

			// 从 gRPC metadata 中获取地址
			if md, ok := metadata.FromIncomingContext(ctx); ok {
				if addresses := md.Get("user-address"); len(addresses) > 0 {
					address := addresses[0]
					if err := addressHelper.ValidateAddress(address); err == nil {
						// 将地址添加到 context 中
						ctx = addressHelper.SetAddressToContext(ctx, address)
					}
				}
			}
		}

		return handler(ctx, req)
	}
}

// GetAddressFromMetadata 从 gRPC metadata 中获取地址
func GetAddressFromMetadata(ctx context.Context) (string, error) {
	addressHelper := logic.NewAddressHelper()
	return addressHelper.GetAddressFromContext(ctx)
}
