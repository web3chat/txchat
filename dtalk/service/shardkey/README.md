# shardkey 分片密钥服务

手机号登录 / 账号绑定的后端服务，原独立项目 `shardkey-service` 已收进 dtalk module。

## 作用

手机号登录不是新的登录协议：App 用手机号 + 短信验证码取回服务端保存的 S1 分片，
与本地 S2 合成助记词后，仍然走原来的钱包私钥签名登录。IM / 区块链层不需要改动。

App 调用的接口（统一前缀 `/api/v1`，经 nginx 转发）：

| 接口 | 用途 |
|---|---|
| `POST /send-code` | 发验证码（purpose: backup/reset/get/delete） |
| `POST /verify-code` | 单独校验验证码 |
| `POST /store-shard` | 首次保存 S1 分片 |
| `POST /get-shard` | 取回 S1 分片 |
| `POST /get-shard-by-sms` | 把 S1 分片发到本机短信 |
| `POST /reset-shard` | 重置 S1 分片（新助记词/新地址） |
| `POST /check-shard-exists` | 查询手机号是否有分片/绑定 |
| `POST /delete-shard` | 注销分片 |
| `POST /bind-account` | 手机号/微信/小红书/抖音绑定到地址（需 FZM-SIGNATURE） |
| `POST /query-chataddress` | 按账号标识查聊天地址 |
| `POST /query-accounts` | 按聊天地址查绑定账号（需 FZM-SIGNATURE） |
| `GET /health` (also HEAD) | 探活 |

## 配置

`config/shardkey.toml` 里所有密钥都是 `${ENV}` 占位符，真实值由 compose 注入：

| 环境变量 | 说明 |
|---|---|
| `MYSQL_PASSWORD` | MySQL 密码 |
| `SHARDKEY_ENV` | `release`（默认，真实发短信）/ `debug`（mock 短信，只打日志） |
| `SHARDKEY_ENCRYPT_KEY` | 分片密钥落库加密密钥；`envfill.sh` 会生成，生成后不要更换 |
| `SUBMAIL_APP_ID` / `SUBMAIL_APP_KEY` / `SUBMAIL_SIGNATURE` | 赛邮短信 |
| `SHARDKEY_IOS_APP_TOKEN` / `SHARDKEY_ANDROID_APP_TOKEN` | App 端 `X-App-Token` |

`config.example.env` 一类包含真实短信凭证的文件**不要提交**；历史值应视为已泄露并轮换。

## 数据库

表结构统一在 `dtalk/script/mysql/shardkey.sql`，首次部署由 `dtalk_biz.sql` 建表；
已有库手工执行 `shardkey.sql`：

- `shard_keys`：手机号 ↔ 聊天地址 ↔ 加密后的 S1 分片（含 `is_deleted`、`version`、previous_*）
- `user_bindings`：手机号/微信/小红书/抖音 ↔ 聊天地址

服务启动只校验表结构，不再执行 DDL。

## 验证码

验证码存 Redis（key `shardkey:code:<phone>`，`shardkey:send:<phone>` 做发送间隔，
`shardkey:wrong:<phone>` 做错误次数限制），支持多实例与重启。单测使用内存实现。

## 安全约定

- `X-App-Id` / `X-App-Token` 是应用标识，不是强安全凭证；抓包可拿到。
- `bind-account` / `query-accounts` 额外要求 `FZM-SIGNATURE`，并校验 body 地址与签名地址一致。
- 手机号已绑定到其他地址时，`store-shard` / `bind-account` 默认拒绝，避免账号分叉。
- 短信发送有 60s 间隔、验证码错误次数上限；生产环境建议再加 IP/日限额。
- 分片密钥落库前 AES-256-GCM 加密；短信和日志不打印验证码/密钥。

## 已知待办

- 老用户手机号已绑定但无 S1 分片：接口返回 `exists=true, shard_missing=true`，App 需要提示用户
  用助记词登录或上传分片，而不是生成新地址。
- 安全设置页直接绑定手机号（Android `SecuritySettingActivity`）目前只校验签名，未校验手机号归属；
  后续 App 应改为走短信验证后再绑定。
- `query-chataddress` 未做签名校验，属于低敏查询；建议后续加 IP 限流。
