# 群组创建白名单功能使用说明

## 功能概述

本功能实现了群组创建的白名单控制，只有白名单中的地址才能创建群组，并且可以限制每个地址创建群组的最大成员数量。

## 核心组件

### 1. 数据模型

- `model.WhitelistInfo`: 白名单信息结构体
- `model.CheckWhitelistResp`: 白名单检查响应结构体
- `model.ErrGroupCreatePermission`: 白名单权限错误

### 2. 数据访问层

- `dao.WhitelistDAO`: 白名单数据访问接口
- 提供白名单的增删改查功能
- 支持按地址查询白名单信息和人数限制

### 3. 业务逻辑层

- `logic.CreateGroupLogic.checkCreateGroupPermissionWithLimit()`: 白名单权限检查
- `logic.AddressHelper`: 地址获取和验证辅助工具

### 4. 中间件

- `http.WhitelistMiddleware`: HTTP 层地址获取中间件
- `grpc.WhitelistInterceptor`: gRPC 层地址获取拦截器

## 使用方法

### 1. 数据库初始化

首先需要创建白名单表：

```sql
-- 执行 sql/whitelist.sql 中的建表语句
CREATE TABLE IF NOT EXISTS `dtalk_group_whitelist` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `address` varchar(128) NOT NULL COMMENT '用户地址',
  `max_members` int(11) NOT NULL DEFAULT 200 COMMENT '最大群成员数量',
  `is_enabled` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否启用 0:禁用 1:启用',
  `create_time` bigint(20) NOT NULL COMMENT '创建时间(毫秒)',
  `update_time` bigint(20) NOT NULL COMMENT '更新时间(毫秒)',
  `description` varchar(255) DEFAULT '' COMMENT '描述信息',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_address` (`address`),
  KEY `idx_enabled` (`is_enabled`),
  KEY `idx_create_time` (`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='群组创建白名单表';
```

### 2. 添加白名单记录

```go
// 创建白名单记录
whitelistInfo := &model.WhitelistInfo{
    Address:     "0x1234567890abcdef1234567890abcdef12345678",
    MaxMembers:  500,
    IsEnabled:   true,
    Description: "管理员地址",
}

err := service.WhitelistDAO.AddWhitelist(ctx, whitelistInfo)
if err != nil {
    log.Error("添加白名单记录失败", err)
}
```

### 3. HTTP 层集成

在 HTTP 服务器中注册白名单中间件：

```go
// 创建白名单中间件
whitelistMiddleware := http.NewWhitelistMiddleware()

// 注册中间件
http.Handle("/app/create-group", whitelistMiddleware.Middleware(http.HandlerFunc(createGroupHandler)))
```

### 4. gRPC 层集成

在 gRPC 服务器中注册白名单拦截器：

```go
// 创建 gRPC 服务器
server := grpc.NewServer(
    grpc.UnaryInterceptor(grpc.WhitelistInterceptor()),
)

// 注册服务
pb.RegisterGroupServer(server, groupServer)
```

### 5. 地址获取方式

#### 方式1: 从 HTTP 请求头获取

```go
// 在 HTTP 请求头中添加用户地址
headers := map[string]string{
    "X-User-Address": "0x1234567890abcdef1234567890abcdef12345678",
}
```

#### 方式2: 从签名中解析

```go
// 从 FZM-SIGNATURE 头中解析地址
signature := r.Header.Get("FZM-SIGNATURE")
address, err := addressHelper.GetAddressFromSignature(signature)
```

#### 方式3: 从 gRPC metadata 获取

```go
// 在 gRPC 调用时添加 metadata
md := metadata.Pairs("user-address", "0x1234567890abcdef1234567890abcdef12345678")
ctx := metadata.NewOutgoingContext(context.Background(), md)
```

## 配置说明

### 1. 白名单配置

- `address`: 用户地址，支持以太坊地址格式
- `max_members`: 最大群成员数量，0 表示无限制
- `is_enabled`: 是否启用，false 表示禁用
- `description`: 描述信息，用于管理

### 2. 地址验证

- 地址长度至少 20 字符
- 支持以太坊地址格式（0x 开头）
- 支持其他区块链地址格式

## 错误处理

### 1. 白名单权限错误

```go
if !whitelistInfo.IsInWhitelist {
    return nil, model.ErrGroupCreatePermission
}
```

### 2. 人数限制错误

```go
if whitelistInfo.MaxMembers > 0 && len(req.Members)+1 > int(whitelistInfo.MaxMembers) {
    return nil, fmt.Errorf("群成员数量超过白名单限制，最大允许 %d 人", whitelistInfo.MaxMembers)
}
```

### 3. 地址格式错误

```go
if err := addressHelper.ValidateAddress(addr); err != nil {
    return nil, fmt.Errorf("地址格式无效: %w", err)
}
```

## 扩展功能

### 1. 动态白名单管理

可以添加管理接口来动态管理白名单：

```go
// 添加白名单记录
func AddWhitelist(ctx context.Context, req *AddWhitelistReq) (*AddWhitelistResp, error)

// 更新白名单记录
func UpdateWhitelist(ctx context.Context, req *UpdateWhitelistReq) (*UpdateWhitelistResp, error)

// 删除白名单记录
func DeleteWhitelist(ctx context.Context, req *DeleteWhitelistReq) (*DeleteWhitelistResp, error)

// 查询白名单列表
func GetWhitelistList(ctx context.Context, req *GetWhitelistListReq) (*GetWhitelistListResp, error)
```

### 2. 白名单缓存

可以添加 Redis 缓存来提高查询性能：

```go
// 缓存白名单信息
func (w *WhitelistDAO) GetWhitelistFromCache(ctx context.Context, address string) (*model.CheckWhitelistResp, error)

// 更新缓存
func (w *WhitelistDAO) UpdateWhitelistCache(ctx context.Context, address string, info *model.CheckWhitelistResp) error
```

### 3. 白名单统计

可以添加统计功能来监控白名单使用情况：

```go
// 统计白名单使用情况
func (w *WhitelistDAO) GetWhitelistStats(ctx context.Context) (*WhitelistStats, error)
```

## 注意事项

1. **地址获取**: 需要根据实际的认证机制来实现地址获取逻辑
2. **性能优化**: 建议使用缓存来提高白名单查询性能
3. **安全考虑**: 地址验证要严格，防止绕过白名单检查
4. **日志记录**: 建议记录白名单检查的日志，便于问题排查
5. **监控告警**: 建议添加白名单相关的监控和告警

## 测试用例

### 1. 白名单检查测试

```go
func TestWhitelistCheck(t *testing.T) {
    // 测试在白名单中的地址
    // 测试不在白名单中的地址
    // 测试人数限制
}
```

### 2. 地址获取测试

```go
func TestAddressRetrieval(t *testing.T) {
    // 测试从 HTTP 请求头获取地址
    // 测试从签名中解析地址
    // 测试从 gRPC metadata 获取地址
}
```

### 3. 错误处理测试

```go
func TestErrorHandling(t *testing.T) {
    // 测试白名单权限错误
    // 测试人数限制错误
    // 测试地址格式错误
}
```
