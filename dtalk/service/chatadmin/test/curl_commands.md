# 群组管理API Curl命令测试

## 群组管理API测试

### 1. 获取群组列表
```bash
curl -X GET "http://localhost:18020/api/v1/groupmanage/groups/list"
```

**返回数据结构：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "total": 100,
    "list": [
      {
        "groupId": 123456,
        "groupMarkId": "group_mark_001",
        "groupName": "测试群组",
        "groupAvatar": "https://example.com/avatar.jpg",
        "groupMemberNum": 50,
        "groupMaximum": 500,
        "groupIntroduce": "这是一个测试群组",
        "groupStatus": 0,
        "groupOwnerId": "owner_001",
        "groupCreateTime": 1640995200000,
        "groupUpdateTime": 1640995200000,
        "groupJoinType": 1,
        "groupMuteType": 0,
        "groupFriendType": 1,
        "groupAESKey": "aes_key_123",
        "groupPubName": "公开群组名称",
        "groupType": 1
      }
    ]
  }
}
```

**字段说明：**
- `total`: 总记录数
- `list`: 群组信息列表
  - `groupId`: 群组ID
  - `groupMarkId`: 群组标识ID
  - `groupName`: 群组名称
  - `groupAvatar`: 群组头像URL
  - `groupMemberNum`: 当前成员数
  - `groupMaximum`: 最大成员数
  - `groupIntroduce`: 群组介绍
  - `groupStatus`: 群组状态 (0=正常, 1=封禁, 2=解散)
  - `groupOwnerId`: 群主ID
  - `groupCreateTime`: 创建时间(毫秒时间戳)
  - `groupUpdateTime`: 更新时间(毫秒时间戳)
  - `groupJoinType`: 加入类型
  - `groupMuteType`: 禁言类型
  - `groupFriendType`: 好友类型
  - `groupAESKey`: AES密钥
  - `groupPubName`: 公开群组名称
  - `groupType`: 群组类型

### 2. 获取群组统计信息
```bash
curl -X GET "http://localhost:18020/api/v1/groupmanage/groups/stats"
```

**返回数据结构：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "totalGroups": 1000,
    "activeGroups": 800,
    "bannedGroups": 50,
    "dissolvedGroups": 150
  }
}
```

**字段说明：**
- `totalGroups`: 群组总数
- `activeGroups`: 正常状态群组数
- `bannedGroups`: 封禁状态群组数
- `dissolvedGroups`: 解散状态群组数

### 3. 获取特定群组信息
```bash
# 替换 {groupId} 为实际的群组ID
curl -X GET "http://localhost:18020/api/v1/groupmanage/groups/info/{groupId}"
```

**返回数据结构：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "groupId": 123456,
    "groupMarkId": "group_mark_001",
    "groupName": "测试群组",
    "groupAvatar": "https://example.com/avatar.jpg",
    "groupMemberNum": 50,
    "groupMaximum": 500,
    "groupIntroduce": "这是一个测试群组",
    "groupStatus": 0,
    "groupOwnerId": "owner_001",
    "groupCreateTime": 1640995200000,
    "groupUpdateTime": 1640995200000,
    "groupJoinType": 1,
    "groupMuteType": 0,
    "groupFriendType": 1,
    "groupAESKey": "aes_key_123",
    "groupPubName": "公开群组名称",
    "groupType": 1
  }
}
```

**字段说明：** 与群组列表中的单个群组信息结构相同

### 4. 更新群组最大人数
```bash
# 替换 {groupId} 为实际的群组ID
curl -X PUT "http://localhost:18020/api/v1/groupmanage/groups/maximum/{groupId}" \
  -H "Content-Type: application/json" \
  -d '{"groupMaximum": 500}'
```

**请求参数：**
```json
{
  "groupMaximum": 500
}
```

**返回数据结构：**
```json
{
  "code": 200,
  "message": "更新成功"
}
```

### 5. 更新群组状态
```bash
# 替换 {groupId} 为实际的群组ID
# groupStatus: 0=正常, 1=封禁, 2=解散
curl -X PUT "http://localhost:18020/api/v1/groupmanage/groups/status/{groupId}" \
  -H "Content-Type: application/json" \
  -d '{"groupStatus": 0}'
```

**请求参数：**
```json
{
  "groupStatus": 0
}
```

**参数说明：**
- `groupStatus`: 群组状态 (0=正常, 1=封禁, 2=解散)

**返回数据结构：**
```json
{
  "code": 200,
  "message": "更新成功"
}
```

### 6. 更新群组名称
```bash
# 替换 {groupId} 为实际的群组ID
curl -X PUT "http://localhost:18020/api/v1/groupmanage/groups/name/{groupId}" \
  -H "Content-Type: application/json" \
  -d '{"groupName": "新的群组名称"}'
```

**请求参数：**
```json
{
  "groupName": "新的群组名称"
}
```

**参数说明：**
- `groupName`: 群组名称 (必填，不能为空)

**返回数据结构：**
```json
{
  "code": 200,
  "message": "更新成功"
}
```

### 7. 根据状态查询群组
```bash
# 查询正常状态的群组 (status=0)
curl -X GET "http://localhost:18020/api/v1/groupmanage/groups/by-status/0"

# 查询封禁状态的群组 (status=1)
curl -X GET "http://localhost:18020/api/v1/groupmanage/groups/by-status/1"

# 查询解散状态的群组 (status=2)
curl -X GET "http://localhost:18020/api/v1/groupmanage/groups/by-status/2"

# 带分页和搜索的状态查询
curl -X GET "http://localhost:18020/api/v1/groupmanage/groups/by-status/0?page=1&pageSize=10&keyword=测试"
```

**路径参数：**
- `status`: 群组状态 (0=正常, 1=封禁, 2=解散)

**查询参数：**
- `page`: 页码 (可选，默认1)
- `pageSize`: 每页数量 (可选，默认10)
- `keyword`: 搜索关键词 (可选)

**返回数据结构：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "total": 50,
    "list": [
      {
        "groupId": 123456,
        "groupMarkId": "group_mark_001",
        "groupName": "测试群组",
        "groupAvatar": "https://example.com/avatar.jpg",
        "groupMemberNum": 50,
        "groupMaximum": 500,
        "groupIntroduce": "这是一个测试群组",
        "groupStatus": 0,
        "groupOwnerId": "owner_001",
        "groupCreateTime": 1640995200000,
        "groupUpdateTime": 1640995200000,
        "groupJoinType": 1,
        "groupMuteType": 0,
        "groupFriendType": 1,
        "groupAESKey": "aes_key_123",
        "groupPubName": "公开群组名称",
        "groupType": 1
      }
    ]
  }
}
```

**字段说明：** 与群组列表返回结构相同

## 白名单管理API测试

### 8. 获取白名单列表
```bash
curl -X GET "http://localhost:18020/api/v1/groupmanage/whitelist/list"
```

**返回数据结构：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "total": 50,
    "list": [
      {
        "id": 1,
        "address": "user_address_123",
        "remark": "测试用户",
        "status": 1,
        "maxMembers": 1000,
        "createdBy": "admin_user_001",
        "createdAt": "2024-01-01T00:00:00Z",
        "updatedAt": "2024-01-01T00:00:00Z"
      }
    ]
  }
}
```

**字段说明：**
- `total`: 总记录数
- `list`: 白名单信息列表
  - `id`: 白名单ID
  - `address`: 用户地址
  - `remark`: 备注信息
  - `status`: 状态 (0=禁用, 1=启用)
  - `maxMembers`: 最大成员数
  - `createdBy`: 创建者
  - `createdAt`: 创建时间
  - `updatedAt`: 更新时间

### 9. 获取白名单统计信息
```bash
curl -X GET "http://localhost:18020/api/v1/groupmanage/whitelist/stats"
```

**返回数据结构：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "totalWhitelist": 100,
    "activeWhitelist": 80,
    "disabledWhitelist": 20
  }
}
```

**字段说明：**
- `totalWhitelist`: 白名单总数
- `activeWhitelist`: 启用状态白名单数
- `disabledWhitelist`: 禁用状态白名单数

### 10. 获取特定白名单用户信息
```bash
# 替换 {whitelistId} 为实际的白名单ID
curl -X GET "http://localhost:18020/api/v1/groupmanage/whitelist/info/{whitelistId}"
```

**返回数据结构：**
```json
{
  "code": 200,
  "message": "success",
  "data": {
    "id": 1,
    "address": "user_address_123",
    "remark": "测试用户",
    "status": 1,
    "maxMembers": 1000,
    "createdBy": "admin_user_001",
    "createdAt": "2024-01-01T00:00:00Z",
    "updatedAt": "2024-01-01T00:00:00Z"
  }
}
```

**字段说明：** 与白名单列表中的单个白名单信息结构相同

### 11. 添加白名单用户
```bash
curl -X POST "http://localhost:18020/api/v1/groupmanage/whitelist/"\
  -H "Content-Type: application/json" \
  -d '{
    "address": "user_address_123",
    "remark": "测试用户",
    "status": 1,
    "maxMembers": 1000,
    "createdBy": "admin_user_001"
  }'
```

**请求参数：**
```json
{
  "address": "user_address_123",
  "remark": "测试用户",
  "status": 1,
  "maxMembers": 1000,
  "createdBy": "admin_user_001"
}
```

**参数说明：**
- `address`: 用户地址 (必填)
- `remark`: 备注信息 (可选)
- `status`: 状态 (可选，默认1=启用)
- `maxMembers`: 最大成员数 (必填，最小值1)
- `createdBy`: 创建者 (必填)

**返回数据结构：**
```json
{
  "code": 200,
  "message": "添加成功"
}
```

### 12. 更新白名单用户
```bash
# 替换 {whitelistId} 为实际的白名单ID
curl -X PUT "http://localhost:18020/api/v1/groupmanage/whitelist/update/{whitelistId}" \
  -H "Content-Type: application/json" \
  -d '{
    "address": "user_address_123",
    "remark": "更新后的测试用户",
    "status": 1,
    "maxMembers": 2000,
    "createdBy": "admin_user_001"
  }'
```

**请求参数：**
```json
{
  "address": "user_address_123",
  "remark": "更新后的测试用户",
  "status": 1,
  "maxMembers": 2000,
  "createdBy": "admin_user_001"
}
```

**参数说明：**
- `address`: 用户地址 (可选)
- `remark`: 备注信息 (可选)
- `status`: 状态 (可选)
- `maxMembers`: 最大成员数 (可选，最小值1)
- `createdBy`: 创建者 (可选)

**返回数据结构：**
```json
{
  "code": 200,
  "message": "更新成功"
}
```

### 13. 删除白名单用户
```bash
# 替换 {whitelistId} 为实际的白名单ID
curl -X DELETE "http://localhost:18020/api/v1/groupmanage/whitelist/delete/{whitelistId}"
```

**返回数据结构：**
```json
{
  "code": 200,
  "message": "删除成功"
}
```

## 分页查询示例

### 群组列表分页查询
```bash
# 第1页，每页10条
curl -X GET "http://localhost:18020/api/v1/groupmanage/groups/list?page=1&pageSize=10"

# 第2页，每页20条
curl -X GET "http://localhost:18020/api/v1/groupmanage/groups/list?page=2&pageSize=20"
```

### 白名单列表分页查询
```bash
# 第1页，每页10条
curl -X GET "http://localhost:18020/api/v1/groupmanage/whitelist/list?page=1&pageSize=10"

# 第2页，每页20条
curl -X GET "http://localhost:18020/api/v1/groupmanage/whitelist/list?page=2&pageSize=20"
```

## 错误处理示例

### 测试无效的群组ID
```bash
curl -X GET "http://localhost:18020/api/v1/groupmanage/groups/info/999999999"
```

**错误返回数据结构：**
```json
{
  "code": 500,
  "message": "获取群组信息失败: group not found"
}
```

### 测试无效的请求参数
```bash
curl -X PUT "http://localhost:18020/api/v1/groupmanage/groups/status/1" \
  -H "Content-Type: application/json" \
  -d '{"groupStatus": 999}'
```

**错误返回数据结构：**
```json
{
  "code": 400,
  "message": "群组状态值无效，应为0-2"
}
```

## 通用错误响应格式

所有API接口在出错时都会返回统一的错误响应格式：

```json
{
  "code": 400,
  "message": "错误描述信息"
}
```

**常见错误码：**
- `400`: 请求参数错误
- `404`: 资源不存在
- `500`: 服务器内部错误

**错误码说明：**
- `400`: 客户端请求参数有误，如参数格式错误、必填参数缺失、参数值超出范围等
- `404`: 请求的资源不存在，如群组ID不存在、白名单ID不存在等
- `500`: 服务器内部错误，如数据库连接失败、SQL执行错误等

## 使用jq美化输出
```bash
# 安装jq (如果未安装)
# Windows: choco install jq
# macOS: brew install jq
# Ubuntu: sudo apt-get install jq

# 使用jq美化JSON输出
curl -s -X GET "http://localhost:18020/api/v1/groupmanage/groups/list" | jq .
```

## 保存响应到文件
```bash
# 保存群组列表到文件
curl -X GET "http://localhost:18020/api/v1/groupmanage/groups/list" -o groups_list.json

# 保存白名单列表到文件
curl -X GET "http://localhost:18020/api/v1/groupmanage/whitelist/list" -o whitelist_list.json
```

## 测试脚本执行权限
```bash
# 给脚本添加执行权限
chmod +x api_tests.sh

# 执行测试脚本
./api_tests.sh
```
