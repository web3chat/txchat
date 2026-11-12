# chatclean 接口说明

本文档描述 **chatclean** 对外 **gRPC** 契约，以及经 **dtalk Gateway** 暴露给 App 的 **HTTP** 形态（路径以 Gateway `SetupEngine` 为准，实际线上可能还有网关前缀，如 `/api/v1` 等，需与运维配置一致）。

---

## 一、通用说明

### 1.1 gRPC（服务间）

| 项 | 说明 |
|----|------|
| 协议 | gRPC / protobuf |
| 包与源码 | `api/chatclean.proto`，Go 包 `github.com/web3chat/txchat/dtalk/service/chatclean/api` |
| 服务名 | `ChatClean` |
| 服务发现 | 客户端通常通过 **etcd**（schema + `SrvName`，如 `chatclean`）解析地址 |
| 鉴权 | **服务间调用无统一 HTTP 头鉴权**；由调用方保证内网与安全策略 |

### 1.2 HTTP（终端 App，经 Gateway）

| 项 | 说明 |
|----|------|
| 方法 | `POST`，`Content-Type: application/json` |
| 认证 | 需通过 Gateway 的 **`AuthMiddleWare`**：请求头携带 **`FZM-SIGNATURE`**，以及约定的 **`FZM-UUID`**、`FZM-DEVICE`、`FZM-DEVICE-NAME`、`FZM-VERSION` 等（与现有 dtalk App 一致） |
| 用户身份 | **`operatorId` 不由客户端传**：由中间件从签名解析地址后写入，对应模型字段为 `json:"-"` |
| 响应封装 | 统一为 `result` + `message` + `data`（见下文错误处理） |

### 1.3 时间字段

- 文中 **Unix ms** 指毫秒级 Unix 时间戳（与消息 `create_time` 一致）。
- **自定义时间范围**仅当 `dateOpt = 0`（CUSTOM）时生效。

---

## 二、错误处理

### 2.1 Gateway HTTP 统一响应

```json
{
  "result": 0,
  "message": "",
  "data": { }
}
```

- **`result == 0`**：业务成功，`data` 为具体业务体。
- **`result != 0`**：失败，`message` 为人类可读说明；部分错误在扩展字段中带有详情（视 `pkg/error` 封装而定）。

### 2.2 常见错误码（HTTP 经 Gateway，`pkg/error`）

| result | 含义 | 常见场景 |
|--------|------|----------|
| `0` | 成功 | — |
| `-1010` | 无效签名 | 缺少/错误 `FZM-SIGNATURE` |
| `-1012` | 请求参数错误 | JSON 绑定失败、`targetId` 为空、`dateOpt=0` 时时间范围非法等 |
| `-1013` | 调用失败 | chatclean gRPC 调用失败等（`message` / ext 中可见下游错误） |

> 说明：chatclean gRPC 若返回 error，Gateway 通常转为 `RPCFailed`（`-1013`）并附带错误信息。

### 2.3 gRPC 错误

- 调用失败时返回标准 gRPC `error`（如下游 MySQL 失败）；**无**与 HTTP `result` 一一对应的统一枚举，需解析 `status` / `message`。

---

## 三、gRPC 接口明细

### 3.1 `CleanPrivateChat`

**说明**：按时间范围删除 `operator_id` 与 `target_id` 之间私聊消息；写审计日志。

**请求 `CleanPrivateChatReq`**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `operator_id` | string | 是 | 发起删除的用户 ID（链上地址等，与存库一致） |
| `target_id` | string | 是 | 会话对方 ID |
| `date_opt` | DateOption | 是 | 时间范围枚举（见下表） |
| `start_time` | int64 | 条件 | **仅 `CUSTOM(0)`** 时有效：起始 Unix ms |
| `end_time` | int64 | 条件 | **仅 `CUSTOM(0)`** 时有效：结束 Unix ms，需 `start_time < end_time` |

**`DateOption` 枚举（与 proto 一致）**

| 值 | 含义 |
|----|------|
| `0` | CUSTOM：使用 `start_time` / `end_time` |
| `1` | 最近 1 天 |
| `2` | 最近 3 天 |
| `3` | 最近 7 天 |
| `4` | 最近 30 天 |
| `5` | 最近 90 天 |
| `6` | 最近 180 天 |
| `7` | 最近 365 天 |
| `8` | 全部（自 0 至当前） |

**响应 `CleanPrivateChatReply`**

| 字段 | 类型 | 说明 |
|------|------|------|
| `deleted_count` | int64 | 删除的 `dtalk_msg_content` 条数（分批累计） |

---

### 3.2 `SetAutoDeleteConfig`

**说明**：为 `(operator_id, target_id)` 会话设置自动删除保留策略（有序 uid 对落库）。

**请求 `SetAutoDeleteConfigReq`**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `operator_id` | string | 是 | 配置发起人 |
| `target_id` | string | 是 | 会话对方 |
| `option` | AutoDeleteOption | 是 | 保留策略（见下表） |

**`AutoDeleteOption` 枚举**

| 值 | 含义 |
|----|------|
| `0` | 不自动删除（`AUTO_NEVER`） |
| `1` | 保留 1 天 |
| `2` | 保留 3 天 |
| `3` | 保留 7 天 |
| `4` | 保留 30 天 |
| `5` | 保留 180 天 |

**响应 `SetAutoDeleteConfigReply`**

| 字段 | 类型 | 说明 |
|------|------|------|
| `option` | AutoDeleteOption | 当前选项 |
| `updated_by` | string | 最后修改人 |
| `updated_at` | int64 | 最后修改时间 Unix ms |

---

### 3.3 `GetAutoDeleteConfig`

**说明**：查询会话的自动删除配置；**若无记录，语义为 `AUTO_NEVER(0)`**。

**请求 `GetAutoDeleteConfigReq`**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `operator_id` | string | 是 | 查询者 |
| `target_id` | string | 是 | 会话对方 |

**响应 `GetAutoDeleteConfigReply`**

| 字段 | 类型 | 说明 |
|------|------|------|
| `option` | AutoDeleteOption | 当前选项；无配置时为 `0` |
| `updated_by` | string | 最后修改人；无配置时可能为空 |
| `updated_at` | int64 | 最后修改时间；无配置时可能为 `0` |

---

## 四、Gateway HTTP 接口（App）

以下路径来自 `gateway/api/v1/internal/handler/routes.go` 与 handler 注释（根路由组无前缀时即为下列路径）。

### 4.1 手动删除私聊

- **路径**：`POST /app/record/clean-private`
- **请求体**（JSON）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `targetId` | string | 是 | 对方用户 ID |
| `dateOpt` | int32 | 是 | 与 `DateOption` 一致 |
| `startTime` | int64 | 条件 | `dateOpt=0` 时必填且与 `endTime` 组成合法区间 |
| `endTime` | int64 | 条件 | `dateOpt=0` 时必填 |

- **成功 `data`**

| 字段 | 类型 | 说明 |
|------|------|------|
| `deletedCount` | int64 | 删除条数 |

**示例请求**

```http
POST /app/record/clean-private HTTP/1.1
Content-Type: application/json
FZM-SIGNATURE: <签名>
FZM-UUID: <设备 UUID>
FZM-DEVICE: Android
FZM-DEVICE-NAME: MyPhone
FZM-VERSION: 1.0.0

{"targetId":"1G5Bg6w4srw965M1JSYTFNtDg4fXpeYh3r","dateOpt":3}
```

**示例响应**

```json
{
  "result": 0,
  "message": "",
  "data": {
    "deletedCount": 42
  }
}
```

---

### 4.2 设置自动删除

- **路径**：`POST /app/record/auto-delete/set`
- **请求体**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `targetId` | string | 是 | 对方用户 ID |
| `option` | int8 | 是 | `AutoDeleteOption` |

- **成功 `data`**

| 字段 | 类型 | 说明 |
|------|------|------|
| `option` | int8 | 当前选项 |
| `updatedBy` | string | 修改人地址 |
| `updatedAt` | int64 | Unix ms |

**示例请求**

```json
{"targetId":"1G5Bg6w4srw965M1JSYTFNtDg4fXpeYh3r","option":3}
```

**示例响应**

```json
{
  "result": 0,
  "message": "",
  "data": {
    "option": 3,
    "updatedBy": "1FQAfBEojcBzKBxaWekyJuc4ZR13N19LtB",
    "updatedAt": 1776921062614
  }
}
```

---

### 4.3 查询自动删除

- **路径**：`POST /app/record/auto-delete/get`
- **请求体**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `targetId` | string | 是 | 对方用户 ID |

- **成功 `data`**：与 4.2 响应 `data` 结构相同（无配置时 `option` 为 `0`）。

**示例请求**

```json
{"targetId":"1G5Bg6w4srw965M1JSYTFNtDg4fXpeYh3r"}
```

---

## 五、特点小结

- **私聊维度**：仅处理两名用户之间的私聊消息，依赖消息库中 relation / content 表。
- **双通道**：人工删除走 RPC **立即执行**；自动删除依赖 **调度器** 周期执行。
- **幂等与数据安全**：删除按批事务处理；生产环境应配合备份、审计与权限控制。
- **与「查询聊天记录」分离**：历史拉取仍走 store / gateway 其它接口，不在 chatclean 服务内。

---

## 六、内部客户端调用示例（Go）

```go
import (
  "context"
  chatclean "github.com/web3chat/txchat/dtalk/service/chatclean/api"
)

// etcdAddr, schema, srvName, dial 与 gateway 配置对齐
cli := chatclean.New(etcdAddr, schema, "chatclean", dial)
deleted, err := cli.CleanPrivateChat(ctx, operatorID, targetID,
  chatclean.DateOption_LAST_7_DAYS, 0, 0)
```

自定义时间范围时使用 `DateOption_CUSTOM` 并传入 `startTime`、`endTime`（ms）。
