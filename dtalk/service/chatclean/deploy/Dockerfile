# chatclean 部署镜像
# 使用前先执行 make build，将二进制文件复制到 deploy/ 目录下
# 示例: cp build/chatclean_v0.1.0 deploy/chatclean

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

RUN adduser -D -H -s /sbin/nologin appuser

WORKDIR /app
RUN mkdir -p /app/etc && chown -R appuser:appuser /app

ARG BINARY_NAME=chatclean
COPY --chown=appuser:appuser ${BINARY_NAME} ./chatclean
RUN chmod +x ./chatclean

USER appuser

EXPOSE 30010

ENTRYPOINT ["./chatclean"]
CMD ["-conf", "/app/etc/chatclean.toml"]
