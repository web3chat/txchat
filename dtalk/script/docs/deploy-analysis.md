# 部署逻辑分析与「记录客户端登录 IP」方案

## 一、script 部署逻辑简要分析

### 1. 构建（script/builder/builder.sh）

- **入参**：`$1` 目标目录，`$2` 目标系统架构（如 `linux_amd64`），`$3` 服务列表（空格分隔）。
- **流程**：
  - 解析 OS/Arch，设置 `GOOS`/`GOARCH`，注入版本/commit/buildTime 等 ldflags。
  - 按服务名 case 选择**配置路径**和**入口 main**：
    - **gateway**：配置 `gateway/api/v1/etc`，入口 `gateway/api/v1/gateway.go`。
    - answer / pusher / store：配置在 `service/record/{name}/config`，入口在 `service/record/{name}/cmd/main.go`。
    - 其他服务：配置 `service/{name}/config`，入口 `service/{name}/cmd/main.go`。
  - 每个服务：拷贝对应 `{serviceName}.toml` 到目标目录，执行 `go build -o {targetDir}/{serviceName} {main}`。
- **结论**：gateway 是独立二进制，配置与代码路径固定，构建脚本不涉及“登录 IP”逻辑。

### 2. 运行时编排（script/compose）

- **service.compose.yaml**：定义业务服务（gateway、backend、device、store、answer 等），gateway 挂载配置卷、加入 `txchat-components` 与 `txchat-service` 网络，**无**在 compose 内再包一层代理。
- **components.compose.yaml**：包含 **nginx**、mysql、minio 等。nginx 使用自定义镜像、挂载配置卷，**依赖 gateway**（`depends_on: gateway`），并暴露 80/8888。

### 3. 实际登录请求链（已同步确认）

当前真实流量路径为：

```
客户端 → Nginx → IM logic 服务（im/ 下）→ HTTP 转发 → Gateway（/user/login 或 /eth/login）
```

即：**客户端先连 Nginx，再连 IM logic；登录请求由 IM logic 通过 HTTP 转发给 gateway**，gateway 收到的是 **IM logic 发起的请求**。

因此：

- 对 **Gateway** 而言，直接 TCP 连接来自 **IM logic 服务**，`RemoteAddr` / 未传头时 `c.ClientIP()` 得到的是 **IM logic 的 IP**，不是终端用户 IP。
- 要记录**真实客户端 IP**，必须由 **IM logic 在转发请求到 gateway 时**，把从 Nginx 得到的真实用户 IP 写入请求头（如 `X-Real-IP`、`X-Forwarded-For` 或约定头 `X-Real-Client-IP`），Gateway 再优先从该头读取。

### 4. 登录发生位置

- **HTTP 登录接口**：`/user/login`、`/eth/login` 在 **gateway** 的 `SetupEngine` 中注册，处理函数为 `account.AddressLogin`。
- **流程**：鉴权中间件解析 `FZM-SIGNATURE` 得到用户地址 → 可选重连冲突校验 → 返回 `Address`。登录请求的**直接调用方**是 IM logic；真实用户 IP 需由 IM logic 通过请求头传给 gateway。
- **结论**：在 gateway 登录成功分支里记录 IP 时，应优先读取 **IM logic 转发的真实用户 IP 头**，再回退到 `c.ClientIP()`（否则会记成 IM logic 的 IP）。

---

## 二、记录客户端登录 IP 的解决方案

### 目标

在用户调用 `/user/login` 或 `/eth/login` 且**登录成功**时，记录该次请求的**客户端 IP**（以及可选：uid、设备类型、uuid、登录时间），便于安全审计、风控或运营统计。

### 方案要点

1. **在 Gateway 获取客户端 IP（适配「客户端 → Nginx → IM logic → Gateway」链）**
   - Gateway 的**直接**请求方是 IM logic，因此：
     - 优先读取 **IM logic 转发的真实用户 IP**：若 IM logic 在转发时设置了约定头（如 `X-Real-Client-IP`），则 gateway 先读该头；
     - 若无约定头，则使用 Gin 的 `c.ClientIP()`（会读 `X-Forwarded-For`、`X-Real-IP`，再回退到 `RemoteAddr`）。此时只要 **IM logic 在转发到 gateway 时** 把从 Nginx 得到的真实用户 IP 写入 `X-Real-IP` 或 `X-Forwarded-For`，gateway 即可拿到真实 IP。
   - **IM logic 侧必须**：在代理请求到 gateway 时，把上游（Nginx）传来的真实客户端 IP 写入请求头，例如：
     - `X-Real-IP: <从 Nginx 请求里取到的 client IP>`
     - 或 `X-Forwarded-For: <同上>`
     - 或约定头 `X-Real-Client-IP: <同上>`（gateway 已支持优先读取，见下节）。
   - Nginx 到 IM logic 段需在 Nginx 配置里设置：`proxy_set_header X-Real-IP $remote_addr;` 和 `proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`，以便 IM logic 能拿到真实用户 IP。
   - 若有多层代理，建议在 Gin 中配置 `TrustedProxies`，只信任直接上游（如 IM logic）的 IP，避免伪造头。

2. **在 Gateway 落库（推荐）**
   - 在 **gateway 使用的 MySQL** 中新增表 `user_login_log`（例如：`uid`、`client_ip`、`login_at`、`device_type`、`device_uuid`）。
   - Gateway 内新增 `LoginLogDAO`，提供 `Insert(ctx, uid, clientIP, loginAt, deviceType, deviceUuid)`；与现有 `WhitelistDAO` 共用同一 `Database` 配置与 `GetDB()`。
   - 在 `account.AddressLogin` 中，**在设置 `ReqResult` 且 `ReqError == nil` 之前**，取 `clientIP := c.ClientIP()`，并调用 `LoginLogDAO.Insert(...)`。建议**异步写入**或“先写日志、再写 DB”，失败仅打日志，不改变登录结果，避免因记录失败影响登录。

3. **Gateway 侧已支持约定头 `X-Real-Client-IP`**
   - 在 `account.AddressLogin` 中取 IP 时，优先读取请求头 `X-Real-Client-IP`（由 IM logic 在转发时设置），若无再使用 `c.ClientIP()`。这样 IM logic 可显式传递“真实终端用户 IP”，避免与链上其他代理的 `X-Forwarded-For` 混淆。

4. **Nginx 配置示例（Nginx → IM logic 段）**

   在 Nginx 转发到 IM logic 的 `location` 里设置，以便 IM logic 能拿到真实用户 IP 并再转发给 gateway：

   ```nginx
   proxy_set_header Host $host;
   proxy_set_header X-Real-IP $remote_addr;
   proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
   proxy_set_header X-Forwarded-Proto $scheme;
   ```

5. **不改动 script 构建与 compose 服务定义**
   - 仅增加 gateway 内表结构、DAO、以及登录 handler 内的一小段“取 IP + 写登录日志”逻辑；部署脚本与 compose 文件无需改。

### 实现清单（代码侧）

- [x] 建表 SQL：`gateway/api/v1/sql/user_login_log.sql`
- [x] DAO：`gateway/api/v1/internal/dao/login_log.go`（`LoginLogDAO.Insert`）
- [x] ServiceContext：注入 `LoginLogDAO`
- [x] 登录 Handler：在成功分支取 `c.ClientIP()`，异步调用 `LoginLogDAO.Insert`，失败仅打日志

按上述清单实现后，即可在现有部署（script 构建 + compose + Nginx）下稳定记录客户端登录 IP。

### IM logic 侧约定（转发登录请求到 Gateway 时）

为让 gateway 记录**真实终端用户 IP**，IM logic 在把登录请求 HTTP 转发到 gateway（`/user/login` 或 `/eth/login`）时，请二选一：

- **推荐**：设置请求头 **`X-Real-Client-IP: <真实用户 IP>`**，值从 Nginx 传到 IM 的请求里取（如 `X-Real-IP` 或 `X-Forwarded-For` 最左端）。Gateway 会优先使用该头。
- 或：把真实用户 IP 写入 **`X-Real-IP`** 或 **`X-Forwarded-For`**（Gateway 会通过 `c.ClientIP()` 读取）。

若未设置上述任一，gateway 会记录到的是 **IM logic 服务自身的 IP**，而非终端用户 IP。

### 使用前需执行

在 gateway 使用的 MySQL 库中执行建表脚本：

```bash
mysql -u root -p your_db < gateway/api/v1/sql/user_login_log.sql
```

或与现有 `gateway.toml` 中 `[Database]` 的 dsn 对应库内执行 `user_login_log.sql` 中的建表语句。
