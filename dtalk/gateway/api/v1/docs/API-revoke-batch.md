# 批量撤回消息 API 接口文档

针对 `POST /app/record/revoke-batch` 批量撤回消息的说明。单条撤回接口 `POST /app/record/revoke` 保持不变。

---

## 通用说明

| 项目 | 说明 |
|------|------|
| **协议** | HTTPS（生产）/ HTTP（本地调试） |
| **基础 URL** | 示例：`https://your-gateway.example.com` 或 `http://localhost:18002` |
| **完整路径** | `{baseUrl}/api/v1/app/record/revoke-batch` |
| **Content-Type** | `application/json` |
| **认证方式** | 请求头 `FZM-SIGNATURE`：鉴权签名（或 MOCK 模式下使用 `MOCK` / `MOCK2` 等） |
| **路径前缀** | `/app/record` |

**注意事项：**

- 所有接口需要先通过鉴权中间件，未带有效 `FZM-SIGNATURE` 会返回签名无效错误。
- 操作者 ID 由服务端从鉴权信息中解析，请求体无需传 `operatorId`。
- `logIds` 服务端自动去重；单次最多 **50** 条。
- 同一请求内所有消息须属于 **同一会话**（同一私聊对象或同一群）。
- 批量撤回成功后，每条成功消息分别下发 `SignalRevoke` 信令（与单条撤回一致）。
- 私聊场景下，接收方可撤销对方发给自己的消息；信令 `SignalRevoke.self=false` 表示操作者非原发送者，客户端可据此区分 UI。

---

## 错误处理

### 响应格式

HTTP 状态码一般为 `200`，业务结果在 body 中通过 `result`、`message`、`data` 表示：

```json
{
  "result": 0,
  "message": "",
  "data": { ... }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `result` | int | 业务码，`0` 表示请求已处理（含部分成功） |
| `message` | string | 提示信息 |
| `data` | object | 批量处理结果 |

### 常见错误码

| result | 说明 |
|--------|------|
| `0` | 请求合法且已处理（含部分成功、全部成功） |
| `-1012` | 参数错误（`type` 非法、`logIds` 为空、超过 50 条等） |
| `-1000` | 系统内部错误 |

### 单条失败原因（`data.failed[].reason`）

| reason | 说明 |
|--------|------|
| 权限不足 | 非会话参与方、无群管理权限、超出撤回时限等 |
| 消息不存在 | 服务端找不到该 `logId` |
| 不属于当前会话 | 批量列表中存在不同会话的消息 |

> 单条失败**不会**导致整批 `result` 非 0，须解析 `data.successCount` / `data.failedCount` / `data.failed`。

---

## 批量撤回消息

- **方法**：`POST`
- **路径**：`/app/record/revoke-batch`
- **Header**：`Content-Type: application/json`，`FZM-SIGNATURE: <签名>`

### 请求参数（Body JSON）

| 参数 | 类型 | 必填 | 约束 | 说明 |
|------|------|------|------|------|
| `type` | int | 是 | `0` 或 `1` | `0` = 私聊；`1` = 群聊 |
| `logIds` | int64[] | 是 | 长度 1～50 | 待撤回的消息 ID 列表 |

**示例请求：**

```json
{
  "type": 1,
  "logIds": [100001, 100002, 100003]
}
```

```bash
curl -X POST "https://your-gateway.example.com/api/v1/app/record/revoke-batch" \
  -H "Content-Type: application/json" \
  -H "FZM-SIGNATURE: <签名>" \
  -d '{"type": 1, "logIds": [100001, 100002]}'
```

### 响应参数

**`data` 字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `total` | int | 去重后的请求总数 |
| `successCount` | int | 成功撤回条数 |
| `failedCount` | int | 失败条数 |
| `succeeded` | int64[] | 成功的 `logId` 列表 |
| `failed` | object[] | 失败明细 |
| `failed[].logId` | int64 | 失败的消息 ID |
| `failed[].reason` | string | 失败原因 |

**示例响应 — 部分成功：**

```json
{
  "result": 0,
  "message": "",
  "data": {
    "total": 3,
    "successCount": 2,
    "failedCount": 1,
    "succeeded": [100001, 100002],
    "failed": [
      {
        "logId": 100003,
        "reason": "权限不足"
      }
    ]
  }
}
```

**示例响应 — 全部成功：**

```json
{
  "result": 0,
  "message": "",
  "data": {
    "total": 2,
    "successCount": 2,
    "failedCount": 0,
    "succeeded": [100001, 100002],
    "failed": []
  }
}
```

### 权限规则（与单条撤回一致）

| 场景 | 规则 |
|------|------|
| 私聊 | 发送者或接收者本人，且在撤回有效期内 |
| 群聊 — 自己的消息 | 发送者本人，且在有效期内 |
| 群聊 — 他人消息 | 群主可撤任意成员；管理员可撤普通成员，不可撤群主 |
| 普通群成员 | 只能撤回自己发送的消息 |

---

## 接口列表

| 接口 | 方法 | 路径 | 说明 |
|------|------|------|------|
| 单条撤回 | POST | `/app/record/revoke` | 现有接口，不变 |
| 批量撤回 | POST | `/app/record/revoke-batch` | 本接口 |
