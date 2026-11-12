# 谈信广场（Square）

独立进程：帖子 / 评论 / 点赞 / 阅读数 / 公众号申请与认证 / DeepSeek 审核 / 独立管理台。

不依赖 dtalk、im 的 Go module，不占用 dtalk MySQL。验签算法从 dtalk 拷贝至 `pkg/`。

## 快速开始

未设置 `SQUARE_PG_HOST` 时，本地启动会自动 `docker pull postgres:16-alpine` 并创建（或复用）容器 `txchat-square-pg`。建库账号如下（可用环境变量覆盖，见 `.env.example`）：

| 项 | 环境变量 | 默认值 |
|----|----------|--------|
| 地址 | `SQUARE_PG_HOST` | 自动容器时 `127.0.0.1` |
| 端口 | `SQUARE_PG_PORT` | 自动容器时 `15432` |
| 用户 | `SQUARE_PG_USER` | `square` |
| 密码 | `SQUARE_PG_PASSWORD` | `square` |
| 库名 | `SQUARE_PG_DB` | `square` |

```bash
export SQUARE_ENV=local
export SQUARE_ALLOW_MOCK=true
export SQUARE_BOOTSTRAP_ADMIN_USER=admin
export SQUARE_BOOTSTRAP_ADMIN_PASSWORD=change-me-please-12
# 额外账号（已存在的用户名不会改密码）：用户名:密码:角色，逗号分隔
# export SQUARE_BOOTSTRAP_ADMINS=developer:change-me-please-12:super,operator:change-me-please-12:reviewer
# 改自动建库密码（须在首次创建容器前设置）：
# export SQUARE_PG_USER=square
# export SQUARE_PG_PASSWORD=your-password
# export SQUARE_PG_DB=square
# 已有外部库：
# export SQUARE_PG_HOST=127.0.0.1 SQUARE_PG_PORT=5432 SQUARE_PG_USER=... SQUARE_PG_PASSWORD=...
go run ./cmd/square
# 或: make run
```

只需先起数据库容器：`make pg`。

- C 端：`http://127.0.0.1:18100/square/app/...`，Header `FZM-SIGNATURE: MOCK`
- 运营台：`http://127.0.0.1:18100/square/console/`
- App 生产对接：[`docs/App对接.md`](docs/App对接.md)
- 全量接口（含管理端）：[`docs/API.md`](docs/API.md)

Compose（需已有 `txchat-components` 网络，或 `make compose-up` 会创建）。默认读取同目录 `.env`；没有该文件则用 `docker-compose.yml` 默认值。服务器用 `.env.prod` 时：`make compose-up ENV_FILE=.env.prod`。

```bash
make compose-up
# 或：make compose-up ENV_FILE=.env.prod
```

整机迁移到新服务器（库内帖子/评论等一并打包，图仍在 OSS）：

```bash
# 旧机器
make migrate-pack ENV_FILE=.env.prod
# 得到 dist/square-migrate-YYYYMMDD-HHMMSS.tar.gz，拷到新机器后：
tar -xzf square-migrate-*.tar.gz
cd square-migrate-*
./restore.sh
```

生产必须设置 `SQUARE_PG_HOST`，禁用 MOCK，且 `SQUARE_JWT_SECRET` 至少 32 字符。密钥不要写入仓库。

运营台「运营账号」页可修改当前登录密码，至少 12 位。根管理员密码忘了时，在服务器环境里把 `SQUARE_BOOTSTRAP_ADMIN_PASSWORD` 写成新密码，`SQUARE_ADMIN_PASSWORD_RESET=true`，重启一次。日志出现 `admin password reset from env` 后登录，再把该开关改回 `false`。这个开关只覆盖 `SQUARE_BOOTSTRAP_ADMIN_USER`，不改其他运营账号。
