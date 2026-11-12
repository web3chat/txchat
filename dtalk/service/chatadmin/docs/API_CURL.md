## GroupManage 服务 API 文档（附 curl 示例）

说明
- 基础地址: http://localhost:18020
- 统一前缀: /api/v1/groupmanage
- 所有请求与响应均为 JSON；需要请求体的接口须带上 Content-Type: application/json

环境变量（可选，便于复用）
```bash
BASE_URL="http://localhost:18020"
API_BASE="/api/v1/groupmanage"
```

---

### 1) 获取群组列表
- 方法: GET
- 路径: `/groups/list`
- 查询参数:
  - page: 页码（默认 1）
  - pageSize: 每页数量（默认 10，最大 100）
  - keyword: 关键词搜索（可选）
  - status: 群状态筛选 0|1|2（可选）

示例
```bash
curl -X GET "$BASE_URL$API_BASE/groups/list?page=1&pageSize=10"
```

---

### 2) 获取群组统计信息
- 方法: GET
- 路径: `/groups/stats`

示例
```bash
curl -X GET "$BASE_URL$API_BASE/groups/stats"
```

---

### 3) 获取群组详情
- 方法: GET
- 路径: `/groups/info/{id}`
- 路径参数:
  - id: 群组 ID（int64）

示例
```bash
GROUP_ID=713758770290757632
curl -X GET "$BASE_URL$API_BASE/groups/info/$GROUP_ID"
```

---

### 4) 更新群组最大人数
- 方法: PUT
- 路径: `/groups/maximum/{id}`
- 请求体:
  - groupMaximum: int > 0

示例
```bash
GROUP_ID=713758770290757632
curl -X PUT "$BASE_URL$API_BASE/groups/maximum/$GROUP_ID" \
  -H "Content-Type: application/json" \
  -d '{"groupMaximum": 500}'
```

---

### 5) 更新群组状态
- 方法: PUT
- 路径: `/groups/status/{id}`
- 请求体:
  - groupStatus: 0=正常, 1=封禁, 2=解散

示例
```bash
GROUP_ID=713758770290757632
curl -X PUT "$BASE_URL$API_BASE/groups/status/$GROUP_ID" \
  -H "Content-Type: application/json" \
  -d '{"groupStatus": 0}'
```

---

### 6) 更新群组名称
- 方法: PUT
- 路径: `/groups/name/{id}`
- 请求体:
  - groupName: string 非空

示例
```bash
GROUP_ID=713758770290757632
curl -X PUT "$BASE_URL$API_BASE/groups/name/$GROUP_ID" \
  -H "Content-Type: application/json" \
  -d '{"groupName": "测试群组名称"}'
```

---

### 7) 获取白名单列表
- 方法: GET
- 路径: `/whitelist/list`
- 查询参数（同群组列表）:
  - page, pageSize, keyword, status

示例
```bash
curl -X GET "$BASE_URL$API_BASE/whitelist/list?page=1&pageSize=10"
```

---

### 8) 获取白名单统计信息
- 方法: GET
- 路径: `/whitelist/stats`

示例
```bash
curl -X GET "$BASE_URL$API_BASE/whitelist/stats"
```

---

### 9) 新增白名单用户
- 方法: POST
- 路径: `/whitelist/`
- 请求体:
  - address: string（必填）
  - remark: string（可选）
  - status: 1=启用, 0=禁用
  - maxMembers: int > 0

示例
```bash
curl -X POST "$BASE_URL$API_BASE/whitelist/" \
  -H "Content-Type: application/json" \
  -d '{
    "address": "user_address_123",
    "remark": "测试用户",
    "status": 1,
    "maxMembers": 1000
  }'
```

---

### 10) 获取白名单详情
- 方法: GET
- 路径: `/whitelist/info/{id}`
- 路径参数:
  - id: 白名单记录主键 ID（int64）

示例
```bash
WID=1
curl -X GET "$BASE_URL$API_BASE/whitelist/info/$WID"
```

---

### 11) 更新白名单用户
- 方法: PUT
- 路径: `/whitelist/update/{id}`
- 请求体（字段按需）：
  - address: string
  - remark: string
  - status: 1|0
  - maxMembers: int > 0

示例
```bash
WID=1
curl -X PUT "$BASE_URL$API_BASE/whitelist/update/$WID" \
  -H "Content-Type: application/json" \
  -d '{
    "address": "user_address_123",
    "remark": "更新后的测试用户",
    "status": 1,
    "maxMembers": 2000
  }'
```

---

### 12) 删除白名单用户
- 方法: DELETE
- 路径: `/whitelist/delete/{id}`

示例
```bash
WID=1
curl -X DELETE "$BASE_URL$API_BASE/whitelist/delete/$WID"
```

---

### 附：常见问题
- 返回 400 且提示参数错误：检查 JSON 字段是否命名正确、类型是否匹配；PUT 接口务必携带请求体
- 返回 404：检查 `{id}` 是否存在；或路径是否使用了新的 info/maximum/status/name/update/delete 结构
- PowerShell 中请避免使用 `curl` 别名，建议使用 `Invoke-RestMethod` 或完整路径运行 curl



