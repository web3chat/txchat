# MinIO 桶清理

自建 MinIO 的聊天媒体在桶 `dtalk-minio-bucket`。OSS 服务没有删除已上传对象的接口，腾空间要在 MinIO 上删对象。

删掉的视频、图片、语音、文件，历史消息里的 `mediaUrl` 会打不开，文字消息还在。

## 环境

| 项 | 值 |
| --- | --- |
| 容器 | `txchat-minio` |
| 网络 | `txchat-components` |
| 桶 | `dtalk-minio-bucket` |
| 数据卷 | `txchat-minio-data`，挂在容器 `/data` |
| 控制台 | 宿主机 `9001` |
| API | 容器内 `http://txchat-minio:9000` |

账号是部署时的 `MINIO_ROOT_USER` / `MINIO_ROOT_PASSWORD`（`compose-up.sh` 里用户名默认 `minio`，密码在 `.env`）。

客户端上传的对象键：

```text
chatList/picture/yyyyMMdd/...
chatList/audio/yyyyMMdd/...
chatList/file/yyyyMMdd/...
chatList/video/yyyyMMdd/...
```

`yyyyMMdd` 是上传日期。视频通常占大头。

## 客户端镜像

Docker Hub 上的 `minio/mc` 已下架，`docker pull minio/mc` 会报 `pull access denied`。用 `pgsty/mc`，命令与原来的 `mc` 相同。只起临时容器，不要换正在跑的 `txchat-minio` 镜像。

确认容器和网络：

```bash
docker ps --format '{{.Names}}\t{{.Networks}}' | grep minio
```

期望输出：

```text
txchat-minio    txchat-components
```

进入客户端：

```bash
docker run --rm -it --network txchat-components --entrypoint sh pgsty/mc
```

下面的 `mc` 命令都在这个容器里执行。退出容器不会停掉 `txchat-minio`。

连上 MinIO（地址用容器名，不要用 `127.0.0.1`）：

```bash
mc alias set local http://txchat-minio:9000 '你的用户名' '你的密码'
```

## 看占用

```bash
mc du local/dtalk-minio-bucket
mc du local/dtalk-minio-bucket/chatList/picture
mc du local/dtalk-minio-bucket/chatList/audio
mc du local/dtalk-minio-bucket/chatList/file
mc du local/dtalk-minio-bucket/chatList/video
```

## 删除一个月之前的视频

`--older-than 30d` 按对象修改时间计算，和上传日期一致。先列出将要删除的对象：

```bash
mc find local/dtalk-minio-bucket/chatList/video --older-than 30d
```

确认后再删：

```bash
mc rm --recursive --force --older-than 30d local/dtalk-minio-bucket/chatList/video
```

删完核对：

```bash
mc du local/dtalk-minio-bucket/chatList/video
```

2026-10-08 执行过一次：视频从 `8.0GiB / 496 objects` 降到 `750MiB / 41 objects`。

图片、语音、文件把路径换成对应目录即可，例如：

```bash
mc find local/dtalk-minio-bucket/chatList/file --older-than 30d
mc rm --recursive --force --older-than 30d local/dtalk-minio-bucket/chatList/file
```

天数可改，例如三个月是 `--older-than 90d`。

## 按某一天删除

日期目录名就是 `yyyyMMdd`。先列出再删：

```bash
mc ls local/dtalk-minio-bucket/chatList/video/
mc rm --recursive --force local/dtalk-minio-bucket/chatList/video/20240115/
```

## 未完成的分段上传

中断的大文件分片不出现在普通对象列表里，但会计入桶用量。OSS 的 `/abort-multipart-upload` 只能取消还握着 `key` 和 `uploadId` 的那一次任务。全桶清理：

```bash
mc rm --incomplete --recursive --force local/dtalk-minio-bucket
```

## 不要做的事

- 不要在容器里直接删除 `/data` 下的对象文件，也不要动 `.minio.sys`。元数据会对不上。
- 桶若开了版本控制，普通删除只打删除标记，用量不下降。这时在 `mc rm` 上加 `--versions`。
