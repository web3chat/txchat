# 群组创建白名单功能使用说明（新表结构）

## 功能概述

基于新的 `group_create_whitelist` 表结构，实现了更完善的群组创建白名单控制功能。

## 新表结构特点

### 1. 字段优化
- **address**: 用户地址，varchar(64)，支持更长的地址格式
- **remark**: 备注说明，便于管理
- **status**: 状态字段，1=启用，0=禁用
- **max_members**: 群聊最大人数上限，默认500
- **created_by**: 创建人地址，便于审计
- **created_at/updated_at**: 使用 timestamp 类型，更精确的时间管理

### 2. 索引优化
- **uk_address**: 地址唯一索引
- **idx_status**: 状态索引，提高查询效率
- **idx_max_members**: 人数上限索引
- **idx_status_max_members**: 复合索引，优化状态和人数查询

## 使用方法

### 1. 数据库初始化

```sql
-- 执行建表语句
CREATE TABLE IF NOT EXISTS `group_create_whitelist` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `address` varchar(64) NOT NULL COMMENT '用户地址',
  `remark` varchar(255) DEFAULT NULL COMMENT '备注说明',
  `status` tinyint(4) NOT NULL DEFAULT '1' COMMENT '状态：1=启用，0=禁用',
  `max_members` int(11) NOT NULL DEFAULT 500 COMMENT '群聊最大人数上限',
  `created_by` varchar(64) DEFAULT NULL COMMENT '创建人地址',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_address` (`address`),
  KEY `idx_status` (`status`),
  KEY `idx_max_members` (`max_members`),
  KEY `idx_created_at` (`created_at`),
  KEY `idx_status_max_members` (`status`, `max_members`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='群组创建白名单表';
```

### 2. 添加白名单记录

```go
// 创建白名单记录
whitelistInfo := &model.WhitelistInfo{
    Address:    "1FKxgaEh5fuSm7a35BfUnKYAmradowpiTR",
    Remark:     "管理员账户",
    Status:     1, // 启用
    MaxMembers: 2000,
    CreatedBy:  "system",
}

err := service.WhitelistDAO.AddWhitelist(ctx, whitelistInfo)
if err != nil {
    log.Error("添加白名单记录失败", err)
}
```

### 3. 查询白名单权限

```go
// 检查地址是否在白名单中
whitelistInfo, err := service.WhitelistDAO.CheckWhitelistWithLimit(ctx, address)
if err != nil {
    return err
}

if !whitelistInfo.IsInWhitelist {
    return model.ErrGroupCreatePermission
}

// 检查人数限制
if whitelistInfo.MaxMembers > 0 && memberCount > int(whitelistInfo.MaxMembers) {
    return fmt.Errorf("群成员数量超过白名单限制，最大允许 %d 人", whitelistInfo.MaxMembers)
}
```

### 4. 更新白名单记录

```go
// 更新白名单记录
whitelistInfo := &model.WhitelistInfo{
    Address:    "1FKxgaEh5fuSm7a35BfUnKYAmradowpiTR",
    Remark:     "VIP管理员账户",
    Status:     1,
    MaxMembers: 3000,
}

err := service.WhitelistDAO.UpdateWhitelist(ctx, whitelistInfo)
if err != nil {
    log.Error("更新白名单记录失败", err)
}
```

### 5. 删除白名单记录（软删除）

```go
// 软删除白名单记录
err := service.WhitelistDAO.DeleteWhitelist(ctx, "1FKxgaEh5fuSm7a35BfUnKYAmradowpiTR")
if err != nil {
    log.Error("删除白名单记录失败", err)
}
```

### 6. 查询白名单列表

```go
// 获取白名单列表
list, err := service.WhitelistDAO.GetWhitelistList(ctx, 0, 10)
if err != nil {
    log.Error("查询白名单列表失败", err)
}

for _, info := range list {
    fmt.Printf("地址: %s, 备注: %s, 状态: %d, 最大人数: %d\n", 
        info.Address, info.Remark, info.Status, info.MaxMembers)
}
```

## 业务逻辑集成

### 1. 在 CreateGroup 中集成白名单检查

```go
func (l *CreateGroupLogic) CreateGroup(req *pb.CreateGroupReq) (*pb.CreateGroupResp, error) {
    // 获取用户地址
    addr := l.getAddressFromRequest(req)
    if addr == "" {
        return nil, fmt.Errorf("无法获取用户地址")
    }
    
    // 验证地址格式
    addressHelper := NewAddressHelper()
    if err := addressHelper.ValidateAddress(addr); err != nil {
        return nil, fmt.Errorf("地址格式无效: %w", err)
    }
    
    // 检查白名单权限和人数上限
    whitelistInfo, err := l.checkCreateGroupPermissionWithLimit(addr)
    if err != nil {
        return nil, err
    }
    
    // 检查群成员数量是否超过白名单限制
    if whitelistInfo.MaxMembers > 0 && len(req.Members)+1 > int(whitelistInfo.MaxMembers) {
        return nil, fmt.Errorf("群成员数量超过白名单限制，最大允许 %d 人", whitelistInfo.MaxMembers)
    }
    
    // 继续创建群组的逻辑...
}
```

### 2. 白名单权限检查

```go
func (l *CreateGroupLogic) checkCreateGroupPermissionWithLimit(addr string) (*model.CheckWhitelistResp, error) {
    whitelistInfo, err := l.svc.WhitelistDAO.CheckWhitelistWithLimit(l.ctx, addr)
    if err != nil {
        return nil, err
    }
    
    if !whitelistInfo.IsInWhitelist {
        return nil, model.ErrGroupCreatePermission
    }
    
    return whitelistInfo, nil
}
```

## 管理接口示例

### 1. 添加白名单接口

```go
// HTTP 接口：添加白名单
func AddWhitelist(c *gin.Context) {
    var req struct {
        Address    string `json:"address" binding:"required"`
        Remark     string `json:"remark"`
        MaxMembers int32  `json:"max_members"`
        CreatedBy  string `json:"created_by"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    whitelistInfo := &model.WhitelistInfo{
        Address:    req.Address,
        Remark:     req.Remark,
        Status:     1,
        MaxMembers: req.MaxMembers,
        CreatedBy:  req.CreatedBy,
    }
    
    err := service.WhitelistDAO.AddWhitelist(c.Request.Context(), whitelistInfo)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, gin.H{"message": "添加成功"})
}
```

### 2. 查询白名单接口

```go
// HTTP 接口：查询白名单列表
func GetWhitelistList(c *gin.Context) {
    offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
    limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
    
    list, err := service.WhitelistDAO.GetWhitelistList(c.Request.Context(), offset, limit)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, gin.H{"data": list})
}
```

## 性能优化建议

### 1. 缓存策略
```go
// 可以添加 Redis 缓存来提高查询性能
func (w *WhitelistDAO) GetWhitelistFromCache(ctx context.Context, address string) (*model.CheckWhitelistResp, error) {
    // 从 Redis 中获取缓存
    // 如果缓存不存在，从数据库查询并缓存
}
```

### 2. 批量操作
```go
// 批量添加白名单记录
func (w *WhitelistDAO) BatchAddWhitelist(ctx context.Context, addresses []string, maxMembers int32) error {
    // 批量插入逻辑
}
```

### 3. 统计查询
```go
// 获取白名单统计信息
func (w *WhitelistDAO) GetWhitelistStats(ctx context.Context) (*WhitelistStats, error) {
    // 统计白名单使用情况
}
```

## 注意事项

1. **地址格式**: 确保地址格式正确，建议添加地址格式验证
2. **权限控制**: 只有管理员才能添加/删除白名单记录
3. **审计日志**: 记录白名单操作的审计日志
4. **性能监控**: 监控白名单查询的性能
5. **数据备份**: 定期备份白名单数据

## 测试用例

### 1. 白名单检查测试
```go
func TestWhitelistCheck(t *testing.T) {
    // 测试在白名单中的地址
    // 测试不在白名单中的地址
    // 测试人数限制
    // 测试状态禁用的情况
}
```

### 2. 数据操作测试
```go
func TestWhitelistCRUD(t *testing.T) {
    // 测试添加白名单记录
    // 测试更新白名单记录
    // 测试删除白名单记录
    // 测试查询白名单列表
}
```

这个新的表结构提供了更完善的白名单管理功能，支持更灵活的配置和更好的性能。

