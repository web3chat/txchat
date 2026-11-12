# 聊天记录清理 API 接口文档

针对 `/app/record` 下新增的聊天记录清理、自动删除配置相关接口的说明。

---

## 通用说明

| 项目       | 说明 |
|------------|------|
| **协议**   | HTTPS（生产）/ HTTP（本地调试） |
| **基础 URL** | 示例：`https://your-gateway.example.com` 或 `http://localhost:18002` |
| **Content-Type** | `application/json` |
| **认证方式** | 请求头 `FZM-SIGNATURE`：鉴权签名（或 MOCK 模式下使用 `MOCK` / `MOCK2` 等） |
| **路径前缀** | 所有接口路径以 `/app/record` 为前缀 |

**注意事项：**

- 所有接口需要先通过鉴权中间件，未带有效 `FZM-SIGNATURE` 会返回签名无效错误。
- 操作者 ID 由服务端从鉴权信息中解析，请求体无需传 `operatorId`。
- 时间相关字段均为 **Unix 毫秒时间戳**。

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

| 字段      | 类型   | 说明 |
|-----------|--------|------|
| `result`  | int    | 业务码，`0` 表示成功 |
| `message` | string | 提示信息，失败时包含错误原因 |
| `data`    | object | 成功时的业务数据，失败时可能为空或 null |

### 常见错误码

| result | 说明 |
|--------|------|
| 0 | 成功 |
| 非 0 | 失败，具体以 `message` 及业务错误码定义为准（如参数错误、签名无效、RPC 调用失败等） |

---

## 接口列表

1. [手动删除私聊记录](#1-手动删除私聊记录) `POST /app/record/clean-private`
2. [设置自动删除配置](#2-设置自动删除配置) `POST /app/record/auto-delete/set`
3. [查询自动删除配置](#3-查询自动删除配置) `POST /app/record/auto-delete/get`

---

## 1. 手动删除私聊记录

删除当前用户与指定用户之间的私聊记录（按时间范围）。

### 请求

- **方法**：`POST`
- **路径**：`/app/record/clean-private`
- **Header**：`Content-Type: application/json`，`FZM-SIGNATURE: <签名>`

#### 请求参数（Body JSON）

| 参数        | 类型 | 必填 | 说明 |
|-------------|------|------|------|
| targetId    | string | 是 | 对方用户 ID（与谁之间的聊天记录） |
| dateOpt     | int32  | 否 | 时间范围选项，见下表，默认可用 0 或 1 |
| startTime   | int64  | 条件 | 自定义起始时间（Unix ms），**仅当 dateOpt=0 时有效** |
| endTime     | int64  | 条件 | 自定义结束时间（Unix ms），**仅当 dateOpt=0 时有效** |

**dateOpt 枚举：**

| 值 | 含义 |
|----|------|
| 0 | 自定义（需同时传 startTime、endTime，且 startTime < endTime） |
| 1 | 近 1 天 |
| 2 | 近 3 天 |
| 3 | 近 7 天 |
| 4 | 近 30 天 |
| 5 | 近 90 天 |
| 6 | 近 180 天 |
| 7 | 近 365 天 |
| 8 | 全部 |

#### 请求示例

**示例 1：删除与某用户最近 7 天的记录**

```bash
curl -X POST "https://your-gateway.example.com/app/record/clean-private" \
  -H "Content-Type: application/json" \
  -H "FZM-SIGNATURE: <your-signature>" \
  -d '{
    "targetId": "user_xxx",
    "dateOpt": 3
  }'
```

**示例 2：自定义时间范围（dateOpt=0）**

```bash
curl -X POST "https://your-gateway.example.com/app/record/clean-private" \
  -H "Content-Type: application/json" \
  -H "FZM-SIGNATURE: <your-signature>" \
  -d '{
    "targetId": "user_xxx",
    "dateOpt": 0,
    "startTime": 1709251200000,
    "endTime": 1709337600000
  }'
```

**示例 3：删除与某用户的全部记录**

```bash
curl -X POST "https://your-gateway.example.com/app/record/clean-private" \
  -H "Content-Type: application/json" \
  -H "FZM-SIGNATURE: <your-signature>" \
  -d '{
    "targetId": "user_xxx",
    "dateOpt": 8
  }'
```

#### 响应参数（data）

| 字段          | 类型  | 说明 |
|---------------|-------|------|
| deletedCount  | int64 | 实际删除的消息条数 |

#### 响应示例

**成功：**

```json
{
  "result": 0,
  "message": "",
  "data": {
    "deletedCount": 42
  }
}
```

**失败（例如自定义范围无效）：**

```json
{
  "result": 40001,
  "message": "custom date range invalid",
  "data": null
}
```

---

## 2. 设置自动删除配置

为当前用户与指定用户的私聊设置「自动删除」策略（如 7 天后自动删除）。

### 请求

- **方法**：`POST`
- **路径**：`/app/record/auto-delete/set`
- **Header**：`Content-Type: application/json`，`FZM-SIGNATURE: <签名>`

#### 请求参数（Body JSON）

| 参数     | 类型  | 必填 | 说明 |
|----------|-------|------|------|
| targetId | string | 是 | 对方用户 ID |
| option   | int8  | 是  | 自动删除保留时长选项，见下表 |

**option 枚举：**

| 值 | 含义     |
|----|----------|
| 0 | 不删除   |
| 1 | 1 天后删除 |
| 2 | 3 天后删除 |
| 3 | 7 天后删除 |
| 4 | 30 天后删除 |
| 5 | 180 天后删除 |

#### 请求示例

**设置 7 天后自动删除：**

```bash
curl -X POST "https://your-gateway.example.com/app/record/auto-delete/set" \
  -H "Content-Type: application/json" \
  -H "FZM-SIGNATURE: <your-signature>" \
  -d '{
    "targetId": "user_xxx",
    "option": 3
  }'
```

**关闭自动删除：**

```bash
curl -X POST "https://your-gateway.example.com/app/record/auto-delete/set" \
  -H "Content-Type: application/json" \
  -H "FZM-SIGNATURE: <your-signature>" \
  -d '{
    "targetId": "user_xxx",
    "option": 0
  }'
```

#### 响应参数（data）

| 字段      | 类型   | 说明 |
|-----------|--------|------|
| option    | int8   | 当前生效的选项 |
| updatedBy | string | 最后修改人（用户 ID） |
| updatedAt | int64  | 最后修改时间（Unix ms） |

#### 响应示例

**成功：**

```json
{
  "result": 0,
  "message": "",
  "data": {
    "option": 3,
    "updatedBy": "user_me",
    "updatedAt": 1709337600000
  }
}
```

**失败（如未传 targetId）：**

```json
{
  "result": 40001,
  "message": "target_id is required",
  "data": null
}
```

---

## 3. 查询自动删除配置

查询当前用户与指定用户之间的自动删除配置。

### 请求

- **方法**：`POST`
- **路径**：`/app/record/auto-delete/get`
- **Header**：`Content-Type: application/json`，`FZM-SIGNATURE: <签名>`

#### 请求参数（Body JSON）

| 参数     | 类型   | 必填 | 说明 |
|----------|--------|------|------|
| targetId | string | 是  | 对方用户 ID |

#### 请求示例

```bash
curl -X POST "https://your-gateway.example.com/app/record/auto-delete/get" \
  -H "Content-Type: application/json" \
  -H "FZM-SIGNATURE: <your-signature>" \
  -d '{
    "targetId": "user_xxx"
  }'
```

#### 响应参数（data）

| 字段      | 类型   | 说明 |
|-----------|--------|------|
| option    | int8   | 当前选项（未配置时一般为 0-不删除） |
| updatedBy | string | 最后修改人 |
| updatedAt | int64  | 最后修改时间（Unix ms） |

#### 响应示例

**成功（未配置过，默认不删除）：**

```json
{
  "result": 0,
  "message": "",
  "data": {
    "option": 0,
    "updatedBy": "",
    "updatedAt": 0
  }
}
```

**成功（已配置 7 天删除）：**

```json
{
  "result": 0,
  "message": "",
  "data": {
    "option": 3,
    "updatedBy": "user_me",
    "updatedAt": 1709337600000
  }
}
```

**失败：**

```json
{
  "result": 40001,
  "message": "target_id is required",
  "data": null
}
```

---

## 汇总表

| 接口说明           | 方法 | 路径                         | 主要请求体 |
|--------------------|------|------------------------------|------------|
| 手动删除私聊记录   | POST | `/app/record/clean-private`  | targetId, dateOpt [, startTime, endTime] |
| 设置自动删除配置   | POST | `/app/record/auto-delete/set` | targetId, option |
| 查询自动删除配置   | POST | `/app/record/auto-delete/get` | targetId |

所有接口均需在 Header 中携带 `FZM-SIGNATURE` 完成鉴权。
