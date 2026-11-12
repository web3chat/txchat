#!/usr/bin/env bash
# 在新机器解开迁移包并启动 postgres + square。
set -euo pipefail

ARCHIVE="${1:-${ARCHIVE:-}}"
PG_CONTAINER="${SQUARE_PG_CONTAINER:-txchat-square-pg}"
APP_CONTAINER="${SQUARE_APP_CONTAINER:-txchat-square}"

log() { printf '%s\n' "==> $*"; }
die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "找不到命令 $1"
}

compose_bin() {
  if docker compose version >/dev/null 2>&1; then
    printf '%s' "docker compose"
    return
  fi
  if command -v docker-compose >/dev/null 2>&1; then
    printf '%s' "docker-compose"
    return
  fi
  die "找不到 docker compose / docker-compose"
}

env_get() {
  local key="$1" file="$2" line val
  line="$(grep -E "^${key}=" "$file" 2>/dev/null | tail -n 1 || true)"
  [[ -z "$line" ]] && { printf '%s' "${3:-}"; return 0; }
  val="${line#*=}"
  val="${val%\"}"
  val="${val#\"}"
  val="${val%\'}"
  val="${val#\'}"
  printf '%s' "$val"
}

enter_pack() {
  if [[ -f "./restore.sh" && -f "./docker-compose.yml" && -f "./data/square.sql.gz" && -f "./env/square.env" ]]; then
    PACK="$(pwd)"
    return 0
  fi
  if [[ -z "$ARCHIVE" ]]; then
    die "请在解压后的 square-migrate-* 目录执行 ./restore.sh，或传入归档：./restore.sh /path/to/square-migrate-xxx.tar.gz"
  fi
  [[ -f "$ARCHIVE" ]] || die "归档不存在：$ARCHIVE"
  local tmp
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/square-migrate.XXXXXX")"
  log "解压 $ARCHIVE -> $tmp"
  tar -xzf "$ARCHIVE" -C "$tmp"
  local dir
  dir="$(find "$tmp" -mindepth 1 -maxdepth 1 -type d -name 'square-migrate-*' | head -n 1 || true)"
  [[ -n "$dir" ]] || die "归档内没有 square-migrate-* 目录 archive=$ARCHIVE"
  cd "$dir"
  PACK="$(pwd)"
}

wait_pg() {
  local i
  for i in $(seq 1 60); do
    if docker exec "$PG_CONTAINER" pg_isready >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  die "postgres 未就绪 container=$PG_CONTAINER"
}

need_cmd docker
need_cmd tar
need_cmd gzip
COMPOSE="$(compose_bin)"
enter_pack

ENV_FILE="$PACK/env/square.env"
SQL_FILE="$PACK/data/square.sql.gz"
[[ -f "$ENV_FILE" ]] || die "缺少 $ENV_FILE"
[[ -f "$SQL_FILE" ]] || die "缺少 $SQL_FILE"

PG_USER="$(env_get SQUARE_PG_USER "$ENV_FILE" square)"
PG_PASSWORD="$(env_get SQUARE_PG_PASSWORD "$ENV_FILE" square)"
PG_DB="$(env_get SQUARE_PG_DB "$ENV_FILE" square)"
SQUARE_IMAGE="$(env_get SQUARE_IMAGE "$ENV_FILE")"
PG_IMAGE="$(env_get SQUARE_PG_IMAGE "$ENV_FILE" postgres:16-alpine)"
[[ -n "$SQUARE_IMAGE" ]] || die "env/square.env 缺少 SQUARE_IMAGE"

if [[ -f "$PACK/images/docker-images.tar.gz" ]]; then
  log "导入镜像 $PACK/images/docker-images.tar.gz"
  gzip -dc "$PACK/images/docker-images.tar.gz" | docker load
else
  log "包内无镜像，尝试拉取 $PG_IMAGE（广场镜像 $SQUARE_IMAGE 必须本机已有）"
  docker pull "$PG_IMAGE"
  docker image inspect "$SQUARE_IMAGE" >/dev/null 2>&1 || die "本机没有镜像 $SQUARE_IMAGE，请用带 images/ 的完整迁移包"
fi

docker network inspect txchat-components >/dev/null 2>&1 || docker network create txchat-components
log "停掉旧容器（数据将写入本目录 compose 的 volume）"
docker rm -f "$APP_CONTAINER" "$PG_CONTAINER" >/dev/null 2>&1 || true

log "启动 Postgres image=$PG_IMAGE"
COPYFILE_DISABLE=1 $COMPOSE --env-file "$ENV_FILE" -f "$PACK/docker-compose.yml" up -d postgres
wait_pg

log "导入全库 user=$PG_USER db=$PG_DB"
if ! gzip -dc "$SQL_FILE" | docker exec -e PGPASSWORD="$PG_PASSWORD" -i "$PG_CONTAINER" \
  psql -U "$PG_USER" -d "$PG_DB" -v ON_ERROR_STOP=1 >/tmp/square-restore-psql.log 2>&1; then
  tail -n 40 /tmp/square-restore-psql.log >&2 || true
  die "psql 导入失败 user=$PG_USER db=$PG_DB 详见 /tmp/square-restore-psql.log"
fi

POSTS="$(docker exec -e PGPASSWORD="$PG_PASSWORD" "$PG_CONTAINER" \
  psql -U "$PG_USER" -d "$PG_DB" -tAc "SELECT COUNT(1) FROM square_post" | tr -d '[:space:]')"
COMMENTS="$(docker exec -e PGPASSWORD="$PG_PASSWORD" "$PG_CONTAINER" \
  psql -U "$PG_USER" -d "$PG_DB" -tAc "SELECT COUNT(1) FROM square_comment" | tr -d '[:space:]')"
log "导入后统计 posts=$POSTS comments=$COMMENTS"

log "启动广场 image=$SQUARE_IMAGE"
COPYFILE_DISABLE=1 $COMPOSE --env-file "$ENV_FILE" -f "$PACK/docker-compose.yml" up -d
probe() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsS "http://127.0.0.1:18100/square/healthz" >/dev/null 2>&1
    return
  fi
  docker inspect -f '{{.State.Running}}' "$APP_CONTAINER" 2>/dev/null | grep -q true
}
local_ok=0
for i in $(seq 1 40); do
  if probe; then
    local_ok=1
    break
  fi
  sleep 2
done
if [[ "$local_ok" != "1" ]]; then
  docker logs --tail 80 "$APP_CONTAINER" >&2 || true
  die "广场未探活 http://127.0.0.1:18100/square/healthz container=$APP_CONTAINER image=$SQUARE_IMAGE"
fi

log "完成。运营台 http://127.0.0.1:18100/square/console/  帖子=$POSTS 评论=$COMMENTS"
if [[ -f "$PACK/meta/manifest.txt" ]]; then
  log "打包清单："
  cat "$PACK/meta/manifest.txt"
fi
