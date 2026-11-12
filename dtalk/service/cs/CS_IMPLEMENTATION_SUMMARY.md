# 客服系统实现总结

> 创建时间: 2026-04-03
> 项目: txchat 客服系统（CS）

---

## 一、需求概述

在现有 txchat IM 系统的基础上，增加 Web 端客服工作台：
- App 端用户通过公开的客服地址**添加好友**直接私聊（复用现有私聊通道，无需额外会话管理）
- Web 端客服人员通过**用户名/密码**登录工作台，接收并回复用户消息
- 私钥存储在服务端，前端不接触任何密钥材料

## 二、整体架构

```
┌─────────────────────┐      ┌─────────────────────┐
│   App 用户 (移动端)   │      │  CS Web 工作台 (浏览器) │
│                     │      │                     │
│  通过客服地址加好友    │      │  用户名+密码登录       │
│  直接私聊            │      │  WebSocket 收发消息    │
└──────────┬──────────┘      └──────────┬──────────┘
           │                            │
           │ WebSocket(FZM-SIGNATURE)   │ HTTP(Session JWT) + WebSocket(FZM-SIGNATURE)
           │                            │
      ┌────▼────┐                 ┌─────▼─────┐
      │ im/Comet │◄────────────────│ dtalk/    │
      │ (WS网关) │                 │ Gateway   │
      └────┬────┘                 └─────┬─────┘
           │ gRPC                       │ HTTP
      ┌────▼────┐                 ┌─────▼─────┐
      │im/Logic │                 │ dtalk/    │
      │ (路由)   │                 │ service/cs│
      └────┬────┘                 │ (客服微服务)│
           │ Kafka                └───────────┘
      ┌────▼────┐                       │
      │ dtalk/  │                  ┌────▼────┐
      │ Pusher  │                  │  MySQL  │
      └─────────┘                  │cs_agent │
                                   └─────────┘
```

### 认证流程

```
客服输入用户名+密码
  → CS 服务验证(bcrypt)
  → 签发 JWT session token (24h有效)
  → 返回 session token + 客服信息

前端请求 /cs/token/refresh (带 JWT)
  → CS 服务用存储的私钥生成 FZM-SIGNATURE (120s有效)
  → 返回 FZM-SIGNATURE

前端用 FZM-SIGNATURE 连接 im/Comet WebSocket
前端用 JWT session token 调用 CS 业务 API
```

## 三、文件清单

### 3.1 新增文件

#### 后端 — `dtalk/service/cs/` (客服微服务)

| 文件 | 说明 |
|------|------|
| `cmd/main.go` | 微服务启动入口，HTTP 服务 + 优雅关闭 |
| `config/config.go` | 配置结构：HTTP地址、MySQL、etcd注册、JWT密钥、AppId |
| `config/cs.toml` | 配置文件模板 |
| `model/model.go` | 数据模型：CSAgent、LoginReq/Resp、RefreshTokenResp 等 |
| `dao/dao.go` | 数据库连接初始化 |
| `dao/agent.go` | Agent CRUD：GetAgentList、GetAgentByID、GetAgentByUsername、UpdateAgentStatus |
| `dao/util.go` | 类型转换辅助函数 |
| `service/service.go` | 业务逻辑：Login(bcrypt验证+JWT签发)、ValidateSession、GenerateFZMSignature、GetAgentList/Info、UpdateAgentStatus |
| `server/http/http.go` | HTTP 路由设置：公开接口(/login,/agents) + 认证接口(/agent/info,/agent/status,/token/refresh) |
| `server/http/handler.go` | HTTP 处理函数 + SessionAuthMiddleware(JWT校验中间件) |
| `api/client.go` | 微服务 HTTP 客户端，供 Gateway 调用 |
| `tools/main.go` | 客服账号管理工具：生成密钥对、bcrypt哈希密码、AES加密私钥、输出SQL |
| `tools/README.md` | 工具使用说明文档 |

#### 后端 — Gateway 修改

| 文件 | 说明 |
|------|------|
| `gateway/api/v1/internal/handler/cs/cs.go` | CS 路由处理函数：LoginHandler、RefreshFZMTokenHandler、GetAgentsHandler、GetAgentInfoHandler、UpdateAgentStatusHandler |

#### 前端 — `cs-web/` (客服 Web 工作台)

| 文件 | 说明 |
|------|------|
| `package.json` | 依赖：React 18、Vite 6、protobufjs |
| `tsconfig.json` | TypeScript 配置 |
| `vite.config.ts` | Vite 配置，含 `/api` 代理到 Gateway |
| `index.html` | HTML 入口 |
| `start.sh` | 启动脚本 |
| `src/main.tsx` | React 入口 |
| `src/App.tsx` | 主组件：LoginPage ↔ WorkbenchPage 切换 |
| `src/index.css` | 全局样式 |
| `src/store.ts` | 全局状态：WebSocket 连接实例 |
| `src/lib/protocol.ts` | im/Comet 二进制 WebSocket 协议编解码 |
| `src/lib/ws.ts` | WebSocket 连接管理：认证、心跳、消息收发 |
| `src/lib/api.ts` | Gateway HTTP API 封装：login、refreshFZMToken、getAgentInfo、updateAgentStatus |
| `src/pages/LoginPage.tsx` | 登录页：用户名+密码 → 获取JWT → 获取FZM-SIGNATURE → 连接Comet |
| `src/pages/WorkbenchPage.tsx` | 工作台：左侧对话列表(按消息自动分组) + 右侧聊天区域 |

#### 数据库

| 文件 | 说明 |
|------|------|
| `dtalk/doc/cs_schema.sql` | cs_agent 表DDL：含 username、password_hash、private_key_enc、public_key 等字段 |

### 3.2 修改文件

| 文件 | 修改内容 |
|------|----------|
| `gateway/api/v1/internal/handler/routes.go` | 新增 `/cs/*` 路由组（login, agents, agent/info, agent/status, token/refresh），不走 FZM-SIGNATURE 中间件 |
| `gateway/api/v1/internal/svc/servicecontext.go` | 新增 `CSClient` 字段（CS 微服务 HTTP 客户端） |
| `gateway/api/v1/internal/config/config.go` | 新增 `CSServiceURL` 配置项 |
| `gateway/api/v1/etc/gateway.toml` | 新增 `CSServiceURL = "http://127.0.0.1:18030"` |

## 四、数据库表结构

```sql
CREATE TABLE IF NOT EXISTS cs_agent (
    id              BIGINT       PRIMARY KEY AUTO_INCREMENT,
    agent_id        VARCHAR(64)  NOT NULL UNIQUE  -- 客服的 IM 地址，由私钥派生
    username        VARCHAR(64)  NOT NULL UNIQUE  -- 登录用户名
    password_hash   VARCHAR(128) NOT NULL         -- bcrypt 密码哈希
    private_key_enc VARCHAR(256) NOT NULL         -- AES加密的 secp256k1 私钥(hex)
    public_key      VARCHAR(128) NOT NULL         -- 压缩公钥(hex)
    name            VARCHAR(64)  NOT NULL         -- 显示名称
    avatar          VARCHAR(256) DEFAULT ''       -- 头像 URL
    status          TINYINT      DEFAULT 0        -- 0=离线 1=在线 2=忙碌
    sort_order      INT          DEFAULT 0        -- 展示排序
    created_at      TIMESTAMP    DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP    DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

## 五、API 接口清单

### CS 微服务 (端口 18030)

| 路径 | 认证 | 说明 |
|------|------|------|
| `POST /cs/login` | 无 | 用户名密码登录，返回 JWT session token + 客服信息 |
| `POST /cs/agents` | 无 | 获取所有客服列表（含在线状态） |
| `POST /cs/agent/info` | JWT | 查询当前登录客服的详细信息 |
| `POST /cs/agent/status` | JWT | 更新客服在线状态 |
| `POST /cs/token/refresh` | JWT | 服务端用存储的私钥签发 FZM-SIGNATURE |

### Gateway 代理路由 (端口 18002 或 19000)

Gateway 在 `/api/v1/cs/*` 下透传上述所有接口到 CS 微服务。

## 六、Go 依赖说明

CS 服务使用的关键依赖（均已在 `dtalk/go.mod` 中）：

| 依赖 | 用途 |
|------|------|
| `github.com/golang-jwt/jwt/v5` | JWT 签发与验证 |
| `golang.org/x/crypto/bcrypt` | 密码哈希 |
| `github.com/web3chat/txchat/dtalk/pkg/auth` | FZM-SIGNATURE 生成（`Request` 方法） |
| `github.com/web3chat/txchat/dtalk/pkg/crypt/secp256k1-haltingstate` | secp256k1 签名驱动 |
| `github.com/web3chat/txchat/dtalk/pkg/address` | 公钥→IM地址派生 |
| `github.com/btcsuite/btcd/btcec` | secp256k1 密钥对生成（tools 使用） |

## 七、部署步骤

### 1. 建表

```bash
mysql -u root -p dtalk < dtalk/doc/cs_schema.sql
```

### 2. 生成客服账号

```bash
cd dtalk/service/cs/tools
go build -o cs-tool .

# 生成 AES 密钥
AES_KEY=$(openssl rand -hex 32)

# 创建客服账号
./cs-tool -username cs_xiaomei -password "pass123" -name "客服小美" -sort 1 -aes-key "$AES_KEY"

# 将输出的 SQL 执行到数据库
```

### 3. 启动 CS 微服务

```bash
cd dtalk/service/cs/cmd
# 修改 cs.toml 配置（MySQL、JWTSecret、AppId 等）
go build -o cs-server .
./cs-server -conf cs.toml
```

### 4. 启动 Gateway（确保配置了 CSServiceURL）

```bash
# gateway.toml 中确认:
# CSServiceURL = "http://127.0.0.1:18030"
```

### 5. 启动 Web 工作台

```bash
cd cs-web
./start.sh
# 访问 http://localhost:3000
```

## 八、设计决策记录

| 决策 | 原因 |
|------|------|
| App 用户通过加好友私聊而非 API 获取客服列表 | 最大化复用现有私聊通道，无需额外会话管理接口 |
| CS 作为独立微服务而非集成在 Gateway 中 | 遵循项目现有微服务架构模式（auth, group, store 等） |
| 用户名密码登录而非私钥登录 | 安全性：私钥不暴露给浏览器前端 |
| 服务端代签 FZM-SIGNATURE | 兼容现有 im/Comet 认证机制，无需修改 IM 基础设施 |
| bcrypt 密码哈希 | 业界标准，自带盐值，抗暴力破解 |
| AES-256-GCM 加密私钥 | 即使数据库泄露，私钥也不会明文暴露 |
| JWT session token (24h) | 避免频繁登录，同时 FZM-SIGNATURE 按需刷新(120s过期) |
| Web 端按消息自动分组对话列表 | 无需显式会话管理，收到消息即建立对话 |
