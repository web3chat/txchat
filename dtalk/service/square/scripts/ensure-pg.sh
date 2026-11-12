#!/usr/bin/env bash
# 未指定外部 PostgreSQL 时：拉取 postgres:16-alpine 并创建/复用 txchat-square-pg。
set -euo pipefail

NAME="${SQUARE_PG_CONTAINER:-txchat-square-pg}"
IMAGE="${SQUARE_PG_IMAGE:-postgres:16-alpine}"
VOLUME="${SQUARE_PG_VOLUME:-txchat-square-pg-data}"
PORT="${SQUARE_PG_PORT:-15432}"
USER_NAME="${SQUARE_PG_USER:-square}"
PASSWORD="${SQUARE_PG_PASSWORD:-square}"
DB_NAME="${SQUARE_PG_DB:-square}"

if [[ -n "${SQUARE_PG_HOST:-}" ]]; then
  echo "SQUARE_PG_HOST=${SQUARE_PG_HOST} 已指定，跳过自动创建容器"
  exit 0
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "ERROR: 未指定 SQUARE_PG_HOST，且未找到 docker" >&2
  exit 1
fi

if ! docker image inspect "${IMAGE}" >/dev/null 2>&1; then
  echo "==> pull ${IMAGE}"
  docker pull "${IMAGE}"
fi
docker volume inspect "${VOLUME}" >/dev/null 2>&1 || docker volume create "${VOLUME}" >/dev/null

if docker inspect -f '{{.State.Status}}' "${NAME}" >/dev/null 2>&1; then
  status="$(docker inspect -f '{{.State.Status}}' "${NAME}")"
  if [[ "${status}" != "running" ]]; then
    echo "==> start ${NAME}"
    docker start "${NAME}" >/dev/null
  else
    echo "==> reuse ${NAME}"
  fi
else
  echo "==> run ${NAME} ${IMAGE} :${PORT} user=${USER_NAME} db=${DB_NAME} password=${PASSWORD}"
  docker run -d \
    --name "${NAME}" \
    --restart unless-stopped \
    -e TZ=Asia/Shanghai \
    -e "POSTGRES_USER=${USER_NAME}" \
    -e "POSTGRES_PASSWORD=${PASSWORD}" \
    -e "POSTGRES_DB=${DB_NAME}" \
    -p "${PORT}:5432" \
    -v "${VOLUME}:/var/lib/postgresql/data" \
    "${IMAGE}" >/dev/null
fi

echo -n "==> wait ready"
for _ in $(seq 1 60); do
  if docker exec "${NAME}" pg_isready -U "${USER_NAME}" -d "${DB_NAME}" >/dev/null 2>&1; then
    echo " ok  127.0.0.1:${PORT} user=${USER_NAME} db=${DB_NAME} password=${PASSWORD}"
    exit 0
  fi
  echo -n "."
  sleep 1
done
echo
echo "ERROR: ${NAME} 未在 60s 内就绪" >&2
exit 1
