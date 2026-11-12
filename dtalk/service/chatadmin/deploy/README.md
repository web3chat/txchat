# chatadmin 独立部署

这个目录只用于 **单独部署 chatadmin**。

它不是集成部署。整套 txchat 一起启动时，用仓库里的 `dtalk/script/compose`，不要用这里的 `docker-compose.yml`。集成部署走共享网络和外部配置卷；这里只起一个 chatadmin 容器，自己映射端口，自己带一份配置。

本文件不会启动 MySQL 或 Redis。把 `.env` 里的地址指到已经在跑的实例。容器要能访问这些地址。

## 用法

在上一级 `service/chatadmin` 编出 Linux 二进制，再复制到本目录：

```bash
cd ..
make build_linux
cp build/bin/chatadmin-linux-amd64 deploy/chatadmin
cd deploy
```

若本机就是 Linux amd64，把上面的 `make build_linux` 换成 `make build`，二进制路径换成 `build/bin/chatadmin`。

```bash
cp .env.example .env
# 修改数据库、Redis、JWT_SECRET、CORS_ORIGIN
docker compose up -d --build
```

运营台地址：`http://<主机>:<HTTP_HOST_PORT>/console/`

`.env` 和复制进来的二进制不要提交。

## 和集成部署的区别

| | 本目录 | `dtalk/script/compose` |
| --- | --- | --- |
| 用途 | 只部署 chatadmin | 和网关、MySQL 等一起部署 |
| 网络 | 自建 `chatadmin-net` | 外部网络 `txchat-service` |
| 配置 | `chatadmin-docker.toml` + `.env` | 外部配置卷 |
| 镜像名 | `txchat/chatadmin:standalone` | `txchat-chatadmin:<版本>` |
