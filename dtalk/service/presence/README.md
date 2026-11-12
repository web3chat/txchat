# presence 已独立出仓

本目录代码已迁移至同级项目：

`../presence`（模块路径：`github.com/txchat/presence`）

部署请使用该仓库的 `make init-compose` / `make docker-compose-up`。

dtalk 侧调用方继续使用 `pkg/presenceclient`，配置 `presenceUrl = "http://txchat-presence:18050"` 即可。
