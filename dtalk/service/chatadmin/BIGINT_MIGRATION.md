# BigInt 类型迁移说明

## 概述

将群组ID相关的字段从 `int64` 类型迁移到 `*big.Int` 类型，以匹配数据库中的 `bigint` 类型，并解决JavaScript精度问题。

## 修改的文件

### 1. 业务模型层 (`model/biz/groupinfo.go`)

#### 修改内容
- `GroupInfo.GroupId`: `int64` → `*big.Int`
- `WhitelistInfo.Id`: `int64` → `*big.Int`
- `UpdateGroupRequest.GroupId`: `int64` → `*big.Int`
- `UpdateWhitelistRequest.Id`: `int64` → `*big.Int`
- `DeleteWhitelistRequest.Id`: `int64` → `*big.Int`

#### 代码示例
```go
// 修改前
type GroupInfo struct {
    GroupId int64 `json:"groupId"`
    // ...
}

// 修改后
type GroupInfo struct {
    GroupId *big.Int `json:"groupId"`
    // ...
}
```

### 2. 数据访问层 (`dao/groupinfo.go`)

#### 修改内容
- 添加 `math/big` 导入
- 更新DAO接口方法签名
- 修改 `convertRecordToBiz` 方法处理 `big.Int`

#### 代码示例
```go
// 修改前
func (d *groupInfoDAO) GetGroupInfo(ctx context.Context, groupId int64) (*biz.GroupInfo, error) {
    records, err := d.mysql.Query(_GetGroupInfoByGroupId, groupId)
    // ...
}

// 修改后
func (d *groupInfoDAO) GetGroupInfo(ctx context.Context, groupId *big.Int) (*biz.GroupInfo, error) {
    records, err := d.mysql.Query(_GetGroupInfoByGroupId, groupId.String())
    // ...
}
```

#### 数据转换
```go
// 修改前
if groupId, ok := record["group_id"]; ok {
    groupInfo.GroupId, _ = strconv.ParseInt(groupId, 10, 64)
}

// 修改后
if groupId, ok := record["group_id"]; ok {
    groupInfo.GroupId = new(big.Int)
    groupInfo.GroupId.SetString(groupId, 10)
}
```

### 3. 服务层 (`service/service.go`)

#### 修改内容
- 添加 `math/big` 导入
- 更新所有相关方法签名

#### 代码示例
```go
// 修改前
func (s *Service) GetGroupInfo(ctx context.Context, groupId int64) (*biz.GroupInfo, error) {
    return s.groupInfoDAO.GetGroupInfo(ctx, groupId)
}

// 修改后
func (s *Service) GetGroupInfo(ctx context.Context, groupId *big.Int) (*biz.GroupInfo, error) {
    return s.groupInfoDAO.GetGroupInfo(ctx, groupId)
}
```

### 4. HTTP处理层 (`server/http/handlers.go`)

#### 修改内容
- 添加 `math/big` 导入
- 更新所有handler方法中的ID解析逻辑

#### 代码示例
```go
// 修改前
groupIdStr := c.Param("id")
groupId, err := strconv.ParseInt(groupIdStr, 10, 64)
if err != nil {
    // 错误处理
}

// 修改后
groupIdStr := c.Param("id")
groupId := new(big.Int)
_, ok := groupId.SetString(groupIdStr, 10)
if !ok {
    // 错误处理
}
```

## 技术细节

### 1. Big.Int 使用方式

#### 创建和初始化
```go
// 从字符串创建
groupId := new(big.Int)
groupId.SetString("1234567890123456789", 10)

// 从int64创建
groupId := big.NewInt(12345)
```

#### 比较操作
```go
// 比较大小
if groupId.Cmp(big.NewInt(0)) == 0 {
    // groupId 等于 0
}

if groupId.Cmp(big.NewInt(100)) > 0 {
    // groupId 大于 100
}
```

#### 字符串转换
```go
// 转换为字符串
groupIdStr := groupId.String()

// 转换为int64（如果值在范围内）
if groupId.IsInt64() {
    int64Value := groupId.Int64()
}
```

### 2. JSON序列化

`big.Int` 类型在JSON序列化时会自动转换为字符串，这解决了JavaScript精度问题：

```go
type GroupInfo struct {
    GroupId *big.Int `json:"groupId"`
}

// 序列化后的JSON
{
    "groupId": "1234567890123456789"  // 字符串形式，避免精度丢失
}
```

### 3. 数据库交互

在数据库查询中，`big.Int` 通过 `String()` 方法转换为字符串：

```go
// 查询参数
records, err := d.mysql.Query(_GetGroupInfoByGroupId, groupId.String())

// 更新参数
_, _, err := d.mysql.Exec(_UpdateGroupMaximum, maximum, time.Now().UnixMilli(), groupId.String())
```

## 优势

### 1. 解决JavaScript精度问题
- 前端接收到的ID是字符串格式
- 避免了JavaScript `Number` 类型的精度限制
- 支持任意大小的整数ID

### 2. 数据库类型匹配
- 与MySQL的 `bigint` 类型完全匹配
- 支持64位有符号整数的完整范围
- 避免数据类型转换的精度丢失

### 3. 类型安全
- 编译时类型检查
- 避免隐式类型转换
- 更明确的API接口

## 注意事项

### 1. 空值处理
```go
// 检查big.Int是否为nil
if groupInfo.GroupId != nil {
    // 使用groupInfo.GroupId
}

// 或者使用零值
if groupInfo.GroupId.Cmp(big.NewInt(0)) != 0 {
    // groupId不为0
}
```

### 2. 性能考虑
- `big.Int` 比 `int64` 稍慢
- 对于频繁的数值运算，考虑缓存转换结果
- 在不需要大整数时，可以使用 `int64`

### 3. 向后兼容性
- API接口保持兼容
- JSON格式变化（数字→字符串）
- 前端需要相应调整

## 测试验证

### 1. 单元测试
```go
func TestBigIntConversion(t *testing.T) {
    // 测试字符串转换
    groupId := new(big.Int)
    groupId.SetString("1234567890123456789", 10)
    
    assert.Equal(t, "1234567890123456789", groupId.String())
}
```

### 2. 集成测试
- 测试API接口的ID参数解析
- 测试数据库查询和更新
- 测试JSON序列化和反序列化

### 3. 前端测试
- 验证JavaScript能正确处理大整数ID
- 测试群组编辑和删除功能
- 验证ID显示的一致性

## 迁移检查清单

- [ ] 更新所有相关的结构体定义
- [ ] 修改DAO层的方法签名和实现
- [ ] 更新Service层的方法签名
- [ ] 修改HTTP handlers的ID解析逻辑
- [ ] 更新单元测试
- [ ] 验证API接口功能
- [ ] 测试前端页面功能
- [ ] 更新相关文档

## 回滚方案

如果需要回滚到 `int64` 类型：

1. 恢复所有结构体定义
2. 恢复DAO层方法签名
3. 恢复Service层方法签名
4. 恢复HTTP handlers逻辑
5. 重新编译和测试

但需要注意JavaScript精度问题会重新出现。

