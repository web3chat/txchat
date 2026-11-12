# 接口文档：用户登录历史查询

> 所属服务：`groupmanage`  
> 版本：v1  
> 更新时间：2026-03-18

---

## 接口概述

根据用户链上地址（`chataddress`）查询其历史登录记录，同时关联返回该地址绑定的手机号、微信号、小红书账号等信息。

数据来源：
- 登录记录 → `dtalk_record.dtalk_login_log`
- 账号绑定 → `dtalk.user_bindings`

---

## 基本信息

| 项目 | 内容 |
|---|---|
| **接口路径** | `GET /api/v1/groupmanage/users/login-history` |
| **请求方式** | `GET` |
| **鉴权方式** | Bearer Token（JWT），Header: `Authorization: Bearer <token>` |
| **Content-Type** | `application/json` |

---

## 请求参数

### Query 参数

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `address` | string | ✅ | — | 用户链上地址（chataddress），精确匹配 |
| `page` | int | ❌ | `1` | 登录记录页码，从 1 开始 |
| `pageSize` | int | ❌ | `20` | 每页记录数，最大值为 `100` |
| `startTime` | int64 | ❌ | — | 登录时间范围下限，Unix 毫秒时间戳，0 或不传表示不限 |
| `endTime` | int64 | ❌ | — | 登录时间范围上限，Unix 毫秒时间戳，0 或不传表示不限 |

### 请求示例

```
GET /api/v1/groupmanage/users/login-history?address=0xAbC123&page=1&pageSize=20&startTime=1700000000000&endTime=1710000000000
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

---

## 响应格式

### 通用响应结构

```json
{
  "code": 200,
  "message": "success",
  "data": { ... }
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `code` | int | 业务状态码，`200` 表示成功 |
| `message` | string | 状态描述 |
| `data` | object | 响应数据，见下方 `data` 字段说明 |

---

### `data` 字段说明（`UserLoginWithBinding`）

| 字段 | 类型 | 说明 |
|---|---|---|
| `address` | string | 查询的用户链上地址 |
| `binding` | object \| null | 账号绑定信息，未绑定时为 `null` |
| `logs` | array | 登录记录列表（当前页） |
| `total` | int64 | 符合条件的登录记录总数（用于分页） |
| `page` | int32 | 当前页码 |
| `pageSize` | int32 | 每页记录数 |

---

### `binding` 对象（`UserBinding`）

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | int64 | 绑定记录主键 ID |
| `phone` | string | 绑定的手机号，未绑定时为空字符串 |
| `wechatId` | string | 绑定的微信号，未绑定时为空字符串 |
| `xiaohongshuId` | string | 绑定的小红书账号，未绑定时为空字符串 |
| `chatAddress` | string | 聊天地址（与查询的 `address` 一致） |
| `isDeleted` | int32 | 是否已删除：`0` = 正常，`1` = 已删除（接口仅返回未删除记录） |
| `createdAt` | string | 绑定创建时间，RFC3339 格式，如 `2024-11-14T10:30:00Z` |
| `updatedAt` | string | 绑定最后更新时间，RFC3339 格式 |

---

### `logs` 数组元素（`LoginRecord`）

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | int64 | 登录记录主键 ID |
| `uid` | string | 用户地址（等同于查询的 `address`） |
| `clientIp` | string | 客户端 IP 地址 |
| `loginAt` | int64 | 登录时间，Unix 毫秒时间戳 |
| `deviceType` | string | 设备类型，如 `iOS`、`Android`、`PC` |
| `deviceUuid` | string | 设备唯一标识 |
| `deviceName` | string | 设备型号/名称（客户端上报，如手机品牌型号），未上报时为空 |
| `country` | string | 国家（由 IP 解析得出），未解析时为空 |
| `cityRegion` | string | 城市+区域（由 IP 解析得出），如 `Shanghai, China`，未解析时为空 |
| `isp` | string | 网络服务商（由 IP 解析得出），如 `Chinanet`，未解析时为空 |
| `createdAt` | string | 记录写入时间，RFC3339 格式 |

> `logs` 按 `loginAt` **降序**排列（最新登录在前）。

---

## 响应示例

### 成功：有绑定信息 + 有登录记录

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "address": "0xAbC1230000000000000000000000000000000001",
    "binding": {
      "id": 42,
      "phone": "13800138000",
      "wechatId": "wx_zhangsan",
      "xiaohongshuId": "xhs_zhangsan",
      "chatAddress": "0xAbC1230000000000000000000000000000000001",
      "isDeleted": 0,
      "createdAt": "2024-11-14T10:30:00Z",
      "updatedAt": "2025-01-20T08:15:00Z"
    },
    "logs": [
      {
        "id": 1001,
        "uid": "0xAbC1230000000000000000000000000000000001",
        "clientIp": "203.0.113.10",
        "loginAt": 1710000000000,
        "deviceType": "iOS",
        "deviceUuid": "A1B2C3D4-E5F6-7890-ABCD-EF1234567890",
        "deviceName": "iPhone15,2",
        "country": "China",
        "cityRegion": "Shanghai, China",
        "isp": "Chinanet",
        "createdAt": "2024-03-10T06:00:00Z"
      },
      {
        "id": 998,
        "uid": "0xAbC1230000000000000000000000000000000001",
        "clientIp": "198.51.100.20",
        "loginAt": 1709900000000,
        "deviceType": "Android",
        "deviceUuid": "F9E8D7C6-B5A4-3210-FEDC-BA9876543210",
        "deviceName": "Pixel 7",
        "country": "United States",
        "cityRegion": "Mountain View, California",
        "isp": "Google LLC",
        "createdAt": "2024-03-09T02:13:20Z"
      }
    ],
    "total": 38,
    "page": 1,
    "pageSize": 20
  }
}
```

---

### 成功：无绑定信息

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "address": "0xDEF4560000000000000000000000000000000002",
    "binding": null,
    "logs": [
      {
        "id": 500,
        "uid": "0xDEF4560000000000000000000000000000000002",
        "clientIp": "192.168.1.1",
        "loginAt": 1709800000000,
        "deviceType": "PC",
        "deviceUuid": "desktop-uuid-001",
        "deviceName": "",
        "country": "",
        "cityRegion": "",
        "isp": "",
        "createdAt": "2024-03-08T00:00:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "pageSize": 20
  }
}
```

---

### 成功：地址存在但无登录记录

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "address": "0xNEW0000000000000000000000000000000000003",
    "binding": {
      "id": 10,
      "phone": "18600001111",
      "wechatId": "",
      "xiaohongshuId": "",
      "chatAddress": "0xNEW0000000000000000000000000000000000003",
      "isDeleted": 0,
      "createdAt": "2025-03-01T00:00:00Z",
      "updatedAt": "2025-03-01T00:00:00Z"
    },
    "logs": [],
    "total": 0,
    "page": 1,
    "pageSize": 20
  }
}
```

---

## 错误响应

| HTTP 状态码 | code | 场景 |
|---|---|---|
| `400` | `400` | `address` 参数为空 |
| `401` | `401` | 未携带 Token 或 Token 已过期/无效 |
| `500` | `500` | 数据库查询失败 |

### 错误示例

```json
{
  "code": 400,
  "message": "用户地址不能为空",
  "data": null
}
```

```json
{
  "code": 401,
  "message": "token校验失败: token is expired",
  "data": null
}
```

```json
{
  "code": 500,
  "message": "查询用户登录历史失败: 查询登录日志列表失败: dial tcp: connection refused",
  "data": null
}
```

---

## 分页说明

| 参数 | 规则 |
|---|---|
| `page` 最小值 | `1`，传入 `≤0` 自动修正为 `1` |
| `pageSize` 范围 | `[1, 100]`，传入 `≤0` 自动修正为 `20`，超过 `100` 截断为 `100` |
| 总页数计算 | `ceil(total / pageSize)` |

---

## 数据来源说明

| 字段来源 | 数据表 | 数据库 | 关联字段 |
|---|---|---|---|
| 登录记录 | `dtalk_login_log` | `dtalk_record` | `uid = address` |
| 账号绑定 | `user_bindings` | `dtalk`（正式环境可配置独立库 `authkey` 等） | `chataddress = address AND is_deleted = 0` |

> 登录记录在 `dtalk_record`；账号绑定库可由 groupmanage 配置 `[MySQLAuthKey]` 指向独立库。

---

## 全站登录日志列表

分页检索 **`dtalk_record.dtalk_login_log`** 全表（不按单个地址必选）；可与下列 **`uid`** 多值并行筛选（等价 SQL：`uid IN (...)`，即 OR）。

| 项目 | 内容 |
|---|---|
| **接口路径** | `GET /api/v1/groupmanage/users/login-logs` |
| **鉴权** | Bearer Token（JWT） |

### 请求参数（Query）

| 参数名 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `page` | int | 否 | 默认 `1` |
| `pageSize` | int | 否 | 默认 `20`，最大 `100` |
| `startTime` | int64 | 否 | 登录时间下限（毫秒），`0` 不限 |
| `endTime` | int64 | 否 | 登录时间上限（毫秒），`0` 不限 |
| `uid` | string（可重复） | 否 | 多个 `uid` 表示并行筛选：`?uid=0xa&uid=0xb`；服务端会去重并最多保留前 `50` 个 |

### 请求示例

```
GET /api/v1/groupmanage/users/login-logs?page=1&pageSize=20&startTime=1700000000000
Authorization: Bearer <token>
```

多地址并行：

```
GET /api/v1/groupmanage/users/login-logs?uid=0xAAA&uid=0xBBB&page=1
Authorization: Bearer <token>
```

### 响应 `data`（`AllLoginLogsResponse`）

| 字段 | 类型 | 说明 |
|---|---|---|
| `list` | array | `LoginRecord` 列表 |
| `total` | int64 | 总条数 |
| `page` | int32 | 当前页 |
| `pageSize` | int32 | 每页条数 |

分页规则与单用户登录历史接口一致。

---

## 相关接口

| 接口 | 说明 |
|---|---|
| `GET /api/v1/groupmanage/users/total-count` | 获取系统总用户数 |
| `GET /api/v1/groupmanage/users/today-new-count` | 获取今日新增用户数 |
| `GET /api/v1/groupmanage/whitelist/by-address` | 根据地址查询白名单信息 |
