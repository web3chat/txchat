# chatadmin 独立部署镜像
# 使用前先在 chatadmin 目录执行 make build_linux，把 Linux 二进制复制到本目录：
#   cp ../build/bin/chatadmin-linux-amd64 ./chatadmin
#
# 配置文件从宿主机挂载进来，权限跟随宿主机。这里用 root 启动，避免 600 的配置文件被普通用户读失败。

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app
RUN mkdir -p /app/etc

ARG BINARY_NAME=chatadmin
COPY ${BINARY_NAME} ./chatadmin
RUN chmod +x ./chatadmin

EXPOSE 18020

ENTRYPOINT ["./chatadmin"]
CMD ["-conf", "/app/etc/chatadmin.toml"]
