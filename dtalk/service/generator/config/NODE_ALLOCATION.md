# Snowflake Node 跨集群分配

`mid`/`logId` 为 int64，由 `dtalk/service/generator` 的 Snowflake `Node` 生成。

## 规则

- 每个**独立部署集群**使用互不重叠的 `Node`（或 Node 段）
- 禁止把 mid 改成字符串前缀
- 联邦投递时目标集群**原样使用源 mid**，禁止重新 `GetID`

## 建议分配表

| 集群 clusterId | generator.Node | 说明 |
|----------------|----------------|------|
| cluster-a      | 1              | 默认开发/主集群 |
| cluster-b      | 2              | 第二集群 |
| cluster-c      | 3              | 预留 |

集群内若有多台 generator，应在该集群预留的 Node 段内再细分（需确认 Snowflake 实现的 Node 位宽）。

## 配置位置

各集群 `generator.toml`：

```toml
Node = 1
```
