# Chatclean 项目说明文档

## 项目定位
`chatclean` 是 `dtalk` 体系下的一个消息清理服务，主要负责两类能力：

1. 手动清理指定两名用户之间的私聊消息。
2. 根据会话级自动删除配置，定时清理超出保留期限的历史私聊消息。

它本身不提供 HTTP 页面，而是以 `gRPC` 服务形式对外提供能力，并通过内部调度器按周期执行自动清理任务。

## 目录结构
核心目录与文件如下：

| 路径 | 作用 |
| --- | --- |
| `cmd/main.go` | 程序入口，负责加载配置、初始化日志、启动调度器和 gRPC 服务、处理优雅退出 |
| `config/config.go` | 配置结构定义、默认值和配置文件加载逻辑 |
| `etc/chatclean.toml` | 本地/开发环境配置示例 |
| `dao/dao.go` | MySQL 连接初始化与连接池设置 |
| `dao/clean.go` | 消息删除、审计日志写入、自动删除配置增删改查 |
| `service/service.go` | 业务层，封装手动清理和自动删除逻辑 |
| `scheduler/scheduler.go` | 定时扫描自动删除配置并触发清理 |
| `server/grpc/server.go` | gRPC 服务实现 |
| `api/chatclean.proto` | gRPC 协议定义 |
| `api/client.go` | 调用方客户端封装 |
| `deploy/` | Dockerfile、docker-compose、容器部署配置 |
| `Makefile` | proto 生成、编译等构建脚本 |

## 启动流程
入口文件是 `cmd/main.go`，整体启动顺序如下：

1. 解析命令行参数。
2. 调用 `config.Init()` 加载配置文件。
3. 初始化日志。
4. 创建 `service.Service`，内部会初始化 MySQL 连接。
5. 创建并启动调度器 `scheduler.Start()`。
6. 启动 gRPC 服务。
7. 监听退出信号，在收到 `SIGTERM`/`SIGINT`/`SIGQUIT` 时依次停止调度器、关闭 gRPC 服务、关闭数据库连接。

当前代码里没有 HTTP Server，外部能力主要经由 gRPC 暴露。

## 配置说明
默认示例配置位于 `etc/chatclean.toml`：

```toml
AppId = "dtalk"
Env   = "debug"

[GRPCServer]
  Network = "tcp"
  Addr    = ":30010"

[Reg]
  Schema   = "dtalk"
  SrvName  = "chatclean"
  RegAddrs = "127.0.0.1:2379"

[MySQL]
  Host = "127.0.0.1"
  Port = 3306
  User = "root"
  Pwd  = "123456"
  Db   = "dtalk"

[Scheduler]
  Interval  = "1h"
  BatchSize = 500
```

其中几个关键点：

1. `GRPCServer.Addr` 指定服务监听端口，默认是 `30010`。
2. `MySQL` 是强依赖，服务启动时会直接 `Ping()` 数据库，失败则进程启动失败。
3. `Scheduler.Interval` 指定自动清理扫描间隔，默认每小时执行一次。
4. `Scheduler.BatchSize` 在配置里存在，但当前删除实现里实际使用的是 `dao/clean.go` 中的常量 `batchSize = 500`，配置项并没有真正生效。

## 核心功能

### 1. 手动删除私聊消息
对应业务方法：`service.Service.CleanPrivateChat`

调用方传入：

- 操作人 `operator_id`
- 目标用户 `target_id`
- 时间范围选项 `date_opt`
- 自定义时间范围 `start_time` / `end_time`（仅 `CUSTOM` 时使用）

服务内部会先根据时间选项换算出时间区间，再调用 DAO 删除符合条件的私聊消息，并写入审计日志。

支持的时间选项定义在 `service/service.go` 与 `api/chatclean.proto` 中，包括：

- 最近 1 天
- 最近 3 天
- 最近 7 天
- 最近 30 天
- 最近 90 天
- 最近 180 天
- 最近 365 天
- 全部时间
- 自定义时间区间

### 2. 自动删除配置
对应业务方法：

- `SetAutoDeleteConfig`
- `GetAutoDeleteConfig`

会话级自动删除支持以下保留时长：

- 不自动删除
- 保留 1 天
- 保留 3 天
- 保留 7 天
- 保留 30 天
- 保留 180 天

服务会将一对用户的 UID 规范化排序后落库，避免 `(A, B)` 和 `(B, A)` 出现两条配置。

### 3. 定时自动清理
定时器逻辑位于 `scheduler/scheduler.go`：

1. 每隔一个 `interval` 拉取所有 `option != 0` 的自动删除配置。
2. 对每一对用户执行一次自动删除。
3. 自动删除的逻辑是：删除“早于当前时间减去保留时长”的消息。

例如配置“保留 7 天”，则清理 7 天以前的历史消息，保留最近 7 天消息。

## 数据删除逻辑
核心删除逻辑位于 `dao/clean.go`，采用“分批删除 + 事务”的策略。

### 删除步骤
单批次删除 `cleanBatch()` 的流程如下：

1. 在 `dtalk_msg_relation` 表中，按双方 UID 和时间范围查出本批待删除的 `mid`。
2. 通过事务先删 `dtalk_msg_relation`。
3. 再删 `dtalk_msg_content`，并额外校验 `sender_id`/`receiver_id` 必须是这两名用户之间的私聊消息。
4. 提交事务。

### 为什么要分批
代码中固定每批最多删除 `500` 条，批次之间还会 `sleep 10ms`，目的是：

- 避免一次删除过多数据导致长事务
- 降低锁表/锁行压力
- 降低对线上 MySQL 的瞬时冲击

### 涉及的表
从代码可见，至少会操作这些表：

| 表名 | 用途 |
| --- | --- |
| `dtalk_msg_relation` | 通过会话关系定位待删除消息 |
| `dtalk_msg_content` | 存储实际消息内容，执行真正删除 |
| `dtalk_clean_audit` | 记录手动或自动清理的审计日志 |
| `dtalk_auto_delete_config` | 存储自动删除配置 |

## 对外暴露的 gRPC 接口
协议定义在 `api/chatclean.proto`，服务名为 `ChatClean`，包含 3 个 RPC：

### 1. `CleanPrivateChat`
用于手动删除私聊消息。

请求字段：

- `operator_id`
- `target_id`
- `date_opt`
- `start_time`
- `end_time`

返回字段：

- `deleted_count`

### 2. `SetAutoDeleteConfig`
用于设置自动删除策略。

请求字段：

- `operator_id`
- `target_id`
- `option`

返回字段：

- `option`
- `updated_by`
- `updated_at`

### 3. `GetAutoDeleteConfig`
用于查询自动删除策略。

请求字段：

- `operator_id`
- `target_id`

返回字段：

- `option`
- `updated_by`
- `updated_at`

## 运行与构建
`Makefile` 提供了几个主要目标：

### 安装 proto 生成工具
```bash
make install-tools
```

### 生成 gRPC 代码
```bash
make gen-proto
```

### 编译服务
```bash
make build
```

当前 `build` 目标会：

1. 强制使用 `GOOS=linux GOARCH=amd64`
2. 打开 `GO111MODULE=on`
3. 使用 `GOPROXY=https://goproxy.cn,direct`
4. 输出二进制到 `build/chatclean_v0.1.0`

所以这个 Makefile 主要偏向“构建 Linux 部署包”，不是本机 Darwin 直接运行版。

## Docker 部署
`deploy/` 目录下提供了容器化部署所需文件：

- `Dockerfile`
- `docker-compose.yml`
- `chatclean-docker.toml`
- `.env.example`

从这些文件可以推断，标准部署方式是：

1. 先在宿主机编译出二进制。
2. 将产物放入镜像构建上下文。
3. 通过 `docker-compose` 挂载配置文件并启动服务。

## 依赖关系
从项目代码可以确认的运行依赖如下：

| 依赖 | 是否使用 | 说明 |
| --- | --- | --- |
| MySQL | 是 | 核心依赖，负责读取和删除消息、记录审计、保存自动清理配置 |
| gRPC | 是 | 对外提供 RPC 服务 |
| 定时任务 | 是 | 进程内 `ticker` 实现自动清理 |
| etcd 注册配置 | 配置中存在，但当前入口未看到注册逻辑 | `etc/chatclean.toml` 中有 `Reg`，但 `cmd/main.go` 当前版本未实际使用 |
| Redis | 否 | 当前项目代码中未看到使用 |
| Kafka | 否 | 当前项目代码中未看到使用 |

## 业务流程说明

### 手动清理流程
1. 客户端通过 gRPC 调用 `CleanPrivateChat`。
2. 服务层将两名用户 UID 规范化排序。
3. 根据时间选项计算删除区间。
4. DAO 按批次删除消息关系和消息内容。
5. 写入一条审计日志。
6. 返回删除条数。

### 自动删除流程
1. 客户端设置某个会话的自动删除策略。
2. 调度器定时扫描所有启用自动删除的配置。
3. 对每对 UID 计算应删除的历史范围。
4. 调用和手动删除相同的 DAO 逻辑执行清理。
5. 写入来源为 `scheduler` 的审计日志。

## 风险点与注意事项

### 1. 物理删除，不可回滚
这是直接 `DELETE`，不是软删除。误操作会直接影响线上聊天记录。

建议：

- 上线前先确认操作范围
- 在生产执行前做好数据备份
- 对调用方增加审批/鉴权/审计约束

### 2. `BatchSize` 配置未真正生效
配置文件里有 `Scheduler.BatchSize`，但 `dao/clean.go` 使用的是固定常量 `500`。如果运维只改配置文件，不会改变真实删除批次。

### 3. 审计日志失败不会阻塞主流程
`WriteAuditLog()` 的错误被忽略，意味着：

- 实际删除可能成功
- 但审计表可能没有记录

排障时不能只看审计表。

### 4. 启动对数据库强依赖
MySQL 无法连接时，服务会直接 `panic` 退出，不会以降级模式启动。

### 5. 自动清理任务是串行执行
调度器当前是逐条配置顺序处理。如果配置很多、消息量很大，单轮执行时间可能较长。

### 6. 自动清理上下文超时为 30 分钟
在 `scheduler.doClean()` 中整轮任务共用 `30m` 超时。如果配置量大，后面的任务可能因为超时被跳过或失败。

## 排障建议

出现问题时，建议按下面顺序排查：

1. 检查配置文件是否正确加载，尤其是 `MySQL` 和 `GRPCServer`。
2. 检查数据库连通性，确认 `dtalk` 库可访问。
3. 检查相关表是否存在：
   - `dtalk_msg_relation`
   - `dtalk_msg_content`
   - `dtalk_clean_audit`
   - `dtalk_auto_delete_config`
4. 检查 gRPC 端口是否正常监听，例如 `30010`。
5. 检查日志中是否有：
   - `scheduler: ListNonNullAutoDeleteConfigs`
   - `scheduler: RunAutoDeleteOnce`
   - `server shutdown error`
6. 如果是“删不掉消息”，优先核对：
   - 时间范围是否正确
   - UID 是否传反或与数据不一致
   - 消息是否真的是双方私聊消息

## 总结
`chatclean` 是一个面向内部系统的私聊消息清理服务，职责比较单一，但对数据安全影响很大。它的设计重点在于：

- 通过 gRPC 提供手动清理与配置管理能力
- 通过定时器执行自动清理
- 通过分批事务降低数据库压力
- 通过审计表记录清理行为

从当前实现看，这个项目已经具备基础可用性，但在“配置项是否真正生效”“删除审计可靠性”“大规模配置下的调度效率”方面还有进一步优化空间。
