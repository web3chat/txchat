# API路径更新说明

## 更新内容

由于修复了路由冲突问题，将状态查询API路径从 `/groups/status/:status` 更改为 `/groups/by-status/:status`，需要同步更新相关测试文件。

## 更新的文件

### 1. curl_commands.md
**更新前：**
```bash
curl -X GET "http://localhost:18020/api/v1/groupmanage/groups/status/0"
curl -X GET "http://localhost:18020/api/v1/groupmanage/groups/status/1"
curl -X GET "http://localhost:18020/api/v1/groupmanage/groups/status/2"
```

**更新后：**
```bash
curl -X GET "http://localhost:18020/api/v1/groupmanage/groups/by-status/0"
curl -X GET "http://localhost:18020/api/v1/groupmanage/groups/by-status/1"
curl -X GET "http://localhost:18020/api/v1/groupmanage/groups/by-status/2"
```

### 2. api_tests.sh
**更新前：**
```bash
api_request "GET" "$API_BASE/groups/status/0" "" "200" "查询正常状态群组"
api_request "GET" "$API_BASE/groups/status/1" "" "200" "查询封禁状态群组"
api_request "GET" "$API_BASE/groups/status/2" "" "200" "查询解散状态群组"
```

**更新后：**
```bash
api_request "GET" "$API_BASE/groups/by-status/0" "" "200" "查询正常状态群组"
api_request "GET" "$API_BASE/groups/by-status/1" "" "200" "查询封禁状态群组"
api_request "GET" "$API_BASE/groups/by-status/2" "" "200" "查询解散状态群组"
```

### 3. Web页面 (api.js)
**更新前：**
```javascript
async getGroupsByStatus(status, params = {}) {
    const queryString = new URLSearchParams(params).toString();
    const endpoint = `/groups/status/${status}${queryString ? '?' + queryString : ''}`;
    return await this.request(endpoint);
}
```

**更新后：**
```javascript
async getGroupsByStatus(status, params = {}) {
    const queryString = new URLSearchParams(params).toString();
    const endpoint = `/groups/by-status/${status}${queryString ? '?' + queryString : ''}`;
    return await this.request(endpoint);
}
```

## 路由冲突说明

### 问题原因
原来的路由配置存在冲突：
```go
groups.PUT("/status/:id", updateGroupStatus)      // 更新群组状态
groups.GET("/status/:status", getGroupsByStatus)  // 按状态查询群组
```

两个路由都使用 `/status` 前缀，但参数名不同（`:id` vs `:status`），可能导致路由匹配问题。

### 解决方案
将查询路由改为更明确的路径：
```go
groups.PUT("/status/:id", updateGroupStatus)        // 更新群组状态
groups.GET("/by-status/:status", getGroupsByStatus) // 按状态查询群组
```

## 测试验证

### 1. 测试curl命令
```bash
# 测试正常状态群组查询
curl -X GET "http://localhost:18020/api/v1/groupmanage/groups/by-status/0"

# 测试封禁状态群组查询
curl -X GET "http://localhost:18020/api/v1/groupmanage/groups/by-status/1"

# 测试解散状态群组查询
curl -X GET "http://localhost:18020/api/v1/groupmanage/groups/by-status/2"
```

### 2. 测试脚本
```bash
cd service/chatadmin/test
chmod +x api_tests.sh
./api_tests.sh
```

### 3. 测试Web界面
1. 打开 `index.html`
2. 进入群组管理页面
3. 测试状态筛选功能
4. 验证编辑功能是否正常

## 注意事项

1. **向后兼容性**：旧的 `/groups/status/:status` 路径将不再可用
2. **文档更新**：确保所有相关文档都已更新
3. **客户端更新**：使用这些API的客户端需要相应更新
4. **测试覆盖**：确保所有测试用例都能通过

## 影响范围

- ✅ `curl_commands.md` - 已更新
- ✅ `api_tests.sh` - 已更新  
- ✅ `web/js/api.js` - 已更新
- ✅ `server/http/http.go` - 已更新
- ❌ 其他客户端代码 - 需要手动更新
- ❌ API文档 - 需要手动更新

