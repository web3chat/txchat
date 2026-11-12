# Chatclean 测试设计文档

## 目标
为 `chatclean` 补一套可以长期维护的测试方案，既能覆盖纯业务逻辑，也能覆盖真实 MySQL 上的数据删除行为，避免仅依赖手工联调和线上冒烟。

本方案将测试分成两层：

1. **纯单元测试**：不依赖 MySQL，覆盖时间范围换算、UID 规范化等纯函数逻辑。
2. **集成测试**：依赖独立 MySQL 测试库，覆盖消息删除、自动删除配置、自动清理执行与审计落库。

## 当前现状
当前仓库中已有 `dtalk/test/chatclean/chatclean_test.go`，这是一套通过 gateway HTTP 接口发起请求的端到端联调用例。它适合做联调和环境验证，但不适合作为主要回归测试基线，原因如下：

1. 依赖 gateway、鉴权、数据库、chatclean 服务整体联通。
2. 失败时定位成本高，难以快速区分是 gateway、鉴权、服务发现还是 chatclean 逻辑问题。
3. 缺少示例数据准备与清理机制，复跑稳定性较差。
4. 删除动作是物理删除，若误连非测试环境，风险较高。

因此，本次补充的测试将优先落在 `service/chatclean/service` 层，先覆盖最核心的业务逻辑和数据行为。

## 测试分层

### 第一层：单元测试
目录建议：

- `service/chatclean/service/service_test.go`

测试目标：

1. `dateOptionToRange` 各种时间选项是否返回合理区间。
2. `normalizeUidPair` 是否能稳定把同一对 UID 规整成统一顺序。

特点：

- 不依赖数据库
- 运行快
- 适合每次提交都跑

### 第二层：集成测试
目录建议：

- `service/chatclean/service/service_integration_test.go`

测试目标：

1. `CleanPrivateChat` 是否只删除指定 UID 对、指定时间范围内的消息。
2. `SetAutoDeleteConfig` / `GetAutoDeleteConfig` 是否能正确落库和读取。
3. `RunAutoDeleteOnce` 是否只删除超出保留期的数据。
4. 审计表 `dtalk_clean_audit` 是否被正确写入。

特点：

- 依赖真实 MySQL
- 启动成本高于单元测试
- 但最贴近真实行为

## 运行前提

集成测试要求连接一个**独立测试库**，不要直连生产或共享开发库。

推荐使用以下环境变量：

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `DTALK_CHATCLEAN_TEST_MYSQL_HOST` | `127.0.0.1` | MySQL 地址 |
| `DTALK_CHATCLEAN_TEST_MYSQL_PORT` | `3306` | MySQL 端口 |
| `DTALK_CHATCLEAN_TEST_MYSQL_USER` | `root` | MySQL 用户 |
| `DTALK_CHATCLEAN_TEST_MYSQL_PWD` | `123456` | MySQL 密码 |
| `DTALK_CHATCLEAN_TEST_MYSQL_DB` | `dtalk` | 测试数据库名 |
| `DTALK_CHATCLEAN_RUN_INTEGRATION` | 空 | 设置为 `1` 时才执行集成测试 |

说明：

1. 默认库名仍然写成 `dtalk`，是为了兼容本地已有脚本和现有表结构。
2. 真正执行时，建议单独准备一个测试库，例如 `dtalk_chatclean_test`，再通过环境变量覆盖。
3. 若未设置 `DTALK_CHATCLEAN_RUN_INTEGRATION=1`，集成测试应自动跳过。

## 示例数据准备策略
由于 `chatclean` 删除的是物理数据，测试数据必须做到：

1. **每个测试用例独立造数**
2. **UID 唯一**
3. **mid 唯一**
4. **测试结束后清理**

推荐策略：

1. 每个测试生成唯一前缀，如 `tc_<timestamp>`。
2. 通过前缀构造 `uidA`、`uidB`、`uidC`。
3. 通过时间戳和自增序号构造 `mid`。
4. 对每条私聊消息插入：
   - 一条 `dtalk_msg_content`
   - 两条 `dtalk_msg_relation`

这样就能模拟真实私聊数据，并验证：

1. 删除双方消息时是否正确命中。
2. 不相关用户的消息是否不会被误删。
3. 过期消息和未过期消息是否能区分。

## 覆盖范围

### 单元测试覆盖点

1. `DateOptionCustom` 返回自定义区间。
2. `DateOptionLast7Days` 等预设范围返回值是否接近当前时间窗口。
3. `DateOptionAllTime` 是否返回 `0 ~ now`。
4. `normalizeUidPair("b", "a")` 是否返回 `("a", "b")`。
5. `normalizeUidPair("a", "a")` 是否保持稳定。

### 集成测试覆盖点

1. `CleanPrivateChat`
   - 删除指定时间范围内消息
   - 保留超出时间范围的消息
   - 不删除非目标会话消息
   - 审计日志条数和删除数正确

2. `SetAutoDeleteConfig` / `GetAutoDeleteConfig`
   - 能写入配置
   - 能读回最新配置
   - UID 交换顺序后仍能命中同一条记录

3. `RunAutoDeleteOnce`
   - 只删除过期消息
   - 不删除未过期消息
   - 生成 `source=1` 的审计日志

## 不在本轮范围内
这次先不直接补这些测试：

1. 通过 gateway 的 HTTP 端到端自动化用例重构
2. gRPC server 启动/监听层测试
3. etcd 服务发现测试
4. 调度器长周期 ticker 行为测试

原因是这几类测试依赖更多外部组件，更适合在下一轮作为系统联调或 smoke test 补充。

## 风险与注意事项

1. **不要把集成测试指向生产库**。该服务删除的是消息内容，不可恢复。
2. 当前 `Scheduler.BatchSize` 配置项没有被 DAO 实际读取，集成测试不应依赖它。
3. 审计日志写入失败不会影响主流程，因此测试中应显式断言审计表数据是否存在。
4. 现有 `dtalk/test/chatclean/chatclean_test.go` 仍可保留为联调用例，但不建议把它当成唯一测试保障。

## 推荐执行方式

仅跑单元测试：

```bash
go test ./service/chatclean/service -run TestDateOption -v
```

跑全部单元测试：

```bash
go test ./service/chatclean/service -v
```

跑集成测试：

```bash
DTALK_CHATCLEAN_RUN_INTEGRATION=1 \
DTALK_CHATCLEAN_TEST_MYSQL_HOST=127.0.0.1 \
DTALK_CHATCLEAN_TEST_MYSQL_PORT=3306 \
DTALK_CHATCLEAN_TEST_MYSQL_USER=root \
DTALK_CHATCLEAN_TEST_MYSQL_PWD=123456 \
DTALK_CHATCLEAN_TEST_MYSQL_DB=dtalk_chatclean_test \
go test ./service/chatclean/service -run Integration -v
```

## 输出物
本轮计划补充以下文件：

1. `service/chatclean/CHATCLEAN_TEST_DESIGN.md`
2. `service/chatclean/service/service_test.go`
3. `service/chatclean/service/service_integration_test.go`

这样可以先把 `chatclean` 的核心逻辑测试基线立住，再逐步往 gRPC 层和 gateway 层扩展。
