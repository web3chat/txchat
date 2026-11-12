# TXChat

TXChat 是一个基于 Go 的分布式即时通讯系统。本仓库是源码单仓库，包含四个 Go module：

| 目录 | Module | 说明 |
|------|--------|------|
| `dtalk/` | `github.com/web3chat/txchat/dtalk` | 业务服务：gateway、auth、backend、group、record、call、oss、backup、chatadmin、chatclean、cs 等 |
| `dtalk/service/square/` | `github.com/web3chat/txchat/dtalk/service/square` | 广场服务，独立 module |
| `im/` | `github.com/web3chat/txchat/im` | 接入层：comet 长连接、logic 调度 |
| `imparse/` | `github.com/web3chat/txchat/imparse` | 消息协议解析库 |

`dtalk`、`im`、`imparse` 通过 `go.mod` 中的相对 `replace` 互相引用，克隆后请保持四个目录的相对位置，不要单独复制某一个 module 进行构建。

## 构建

环境要求：

- Go 1.23 及以上（`dtalk` 使用 `go 1.23` + `toolchain go1.24.5`）
- Make、Bash
- Docker Engine 20.10+ 与 Docker Compose（部署时需要）

```bash
cd dtalk
make build        # 本机平台
make build_linux_amd64

cd ../im
make build
make build_linux_amd64
```

前端代码位于 `dtalk/service/web/cs-web`，使用 Vite + React，构建方式见该目录的 `package.json`。

## 部署

完整部署步骤见 [`部署.md`](部署.md)。部署前请阅读其中的目录要求、`run_compose/.env` 密码说明和启动顺序。

## 配置与密钥

- 各服务的示例配置在 `service/*/config/`，Compose 模板在 `dtalk/script/compose/`、`im/script/compose/`。
- `.env.example` 和 `*.toml` 中的密码、JWT 密钥、对象存储密钥均为开发占位值，生产环境必须通过环境变量或 `run_compose/.env` 注入。
- 仓库不包含线上凭证。如果发现示例中仍含真实值，请立即替换并轮换对应密钥。

## 上游与许可

本项目基于 [txchat](https://github.com/txchat) 的 dtalk / im / imparse 项目（MIT License）继续开发，并保留原版权声明。新增和修改部分由 web3chat 维护。

上游依赖 `github.com/txchat/im-pkg` 仍按原 module path 引用。许可全文见 [`LICENSE`](LICENSE)。
