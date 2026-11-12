#!/usr/bin/env bash
# 把当前 compose 部署（镜像 + Postgres 全库含帖子 + 配置）打成一份迁移包。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
log() { printf '%s\n' "==> $*"; }
die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

pick_env() {
  if [[ -n "${1:-}" && -f "$1" ]]; then
    printf '%s' "$1"
    return
  fi
  if [[ -n "${ENV_FILE:-}" && -f "$ENV_FILE" ]]; then
    printf '%s' "$ENV_FILE"
    return
  fi
  if [[ -f "$ROOT/.env.prod" ]]; then
    printf '%s' "$ROOT/.env.prod"
    return
  fi
  if [[ -f "$ROOT/.env" ]]; then
    printf '%s' "$ROOT/.env"
    return
  fi
  die "找不到环境文件，请指定：make migrate-pack ENV_FILE=.env.prod"
}
ENV_FILE="$(pick_env "${1:-}")"
OUT_DIR="${OUT_DIR:-$ROOT/dist}"
PG_CONTAINER="${SQUARE_PG_CONTAINER:-txchat-square-pg}"
APP_CONTAINER="${SQUARE_APP_CONTAINER:-txchat-square}"
SKIP_IMAGES="${SKIP_IMAGES:-0}"
STAMP="$(date +%Y%m%d-%H%M%S)"
NAME="square-migrate-${STAMP}"
WORK="${OUT_DIR}/.${NAME}.work"

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

ensure_pg_running() {
  if docker inspect -f '{{.State.Running}}' "$PG_CONTAINER" 2>/dev/null | grep -q true; then
    return 0
  fi
  if docker inspect "$PG_CONTAINER" >/dev/null 2>&1; then
    log "启动已有容器 $PG_CONTAINER"
    docker start "$PG_CONTAINER" >/dev/null
  else
    die "未找到 $PG_CONTAINER，请先在本机执行 make compose-up（可用 ENV_FILE=$ENV_FILE）"
  fi
  local i
  for i in $(seq 1 40); do
    if docker exec "$PG_CONTAINER" pg_isready >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  die "postgres 未就绪 container=$PG_CONTAINER"
}

pg_exec() {
  docker exec -e PGPASSWORD="$PG_PASSWORD" "$PG_CONTAINER" "$@"
}

cleanup() {
  rm -rf "$WORK"
}
trap cleanup EXIT

need_cmd docker
need_cmd tar
need_cmd gzip
[[ -f "$ENV_FILE" ]] || die "环境文件不存在：$ENV_FILE（服务器一般是 .env.prod）"

PG_USER="$(env_get SQUARE_PG_USER "$ENV_FILE" square)"
PG_PASSWORD="$(env_get SQUARE_PG_PASSWORD "$ENV_FILE" square)"
PG_DB="$(env_get SQUARE_PG_DB "$ENV_FILE" square)"
[[ -n "$PG_USER" && -n "$PG_DB" ]] || die "ENV_FILE=$ENV_FILE 缺少 SQUARE_PG_USER / SQUARE_PG_DB"

log "打包目录 $NAME"
mkdir -p "$WORK/$NAME"/{data,images,env,web,sql,meta}
COPYFILE_DISABLE=1

ensure_pg_running

log "导出 Postgres 全库（含帖子/评论/点赞/关注/举报/运营账号）user=$PG_USER db=$PG_DB container=$PG_CONTAINER"
if ! pg_exec pg_dump -U "$PG_USER" -d "$PG_DB" --clean --if-exists --no-owner --no-acl --encoding=UTF8 \
  | gzip -c >"$WORK/$NAME/data/square.sql.gz"; then
  die "pg_dump 失败 container=$PG_CONTAINER user=$PG_USER db=$PG_DB"
fi
[[ -s "$WORK/$NAME/data/square.sql.gz" ]] || die "导出的 SQL 为空"

POSTS="$(pg_exec psql -U "$PG_USER" -d "$PG_DB" -tAc "SELECT COUNT(1) FROM square_post" 2>/dev/null | tr -d '[:space:]' || printf '0')"
COMMENTS="$(pg_exec psql -U "$PG_USER" -d "$PG_DB" -tAc "SELECT COUNT(1) FROM square_comment" 2>/dev/null | tr -d '[:space:]' || printf '0')"
log "库内统计 posts=$POSTS comments=$COMMENTS"

PG_IMAGE="$(docker inspect -f '{{.Config.Image}}' "$PG_CONTAINER")"
SQUARE_IMAGE=""
if docker inspect "$APP_CONTAINER" >/dev/null 2>&1; then
  SQUARE_IMAGE="$(docker inspect -f '{{.Config.Image}}' "$APP_CONTAINER")"
fi
if [[ -z "$SQUARE_IMAGE" ]]; then
  SQUARE_IMAGE="$(docker images --format '{{.Repository}}:{{.Tag}}' | grep -E '^txchat-square:|^square-square:|^square_square:' | head -n 1 || true)"
fi
[[ -n "$SQUARE_IMAGE" ]] || die "找不到广场镜像，请先 make compose-up 或 make image"

if [[ "$SKIP_IMAGES" != "1" ]]; then
  log "导出 Docker 镜像 pg=$PG_IMAGE square=$SQUARE_IMAGE"
  if ! docker save "$PG_IMAGE" "$SQUARE_IMAGE" | gzip -c >"$WORK/$NAME/images/docker-images.tar.gz"; then
    die "docker save 失败 pg=$PG_IMAGE square=$SQUARE_IMAGE"
  fi
else
  log "SKIP_IMAGES=1，不打包镜像（新机器需能拉取 $PG_IMAGE 并自行准备 $SQUARE_IMAGE）"
fi

cp "$ENV_FILE" "$WORK/$NAME/env/square.env"
{
  printf '\n# --- pack-migrate 写入，restore 时覆盖连库主机 ---\n'
  printf 'SQUARE_PG_IMAGE=%s\n' "$PG_IMAGE"
  printf 'SQUARE_IMAGE=%s\n' "$SQUARE_IMAGE"
} >>"$WORK/$NAME/env/square.env"

cp "$ROOT/script/docker/compose-migrate.yml" "$WORK/$NAME/docker-compose.yml"
cp "$ROOT/sql/schema.sql" "$WORK/$NAME/sql/schema.sql"
cp "$ROOT/scripts/restore-migrate.sh" "$WORK/$NAME/restore.sh"
chmod +x "$WORK/$NAME/restore.sh"
rm -rf "$WORK/$NAME/web/admin"
cp -R "$ROOT/web/admin" "$WORK/$NAME/web/admin"
find "$WORK/$NAME" -name '._*' -delete 2>/dev/null || true

{
  printf 'name=%s\n' "$NAME"
  printf 'packedAt=%s\n' "$(date -Iseconds)"
  printf 'envFile=%s\n' "$ENV_FILE"
  printf 'pgContainer=%s\n' "$PG_CONTAINER"
  printf 'pgImage=%s\n' "$PG_IMAGE"
  printf 'squareImage=%s\n' "$SQUARE_IMAGE"
  printf 'pgUser=%s\n' "$PG_USER"
  printf 'pgDb=%s\n' "$PG_DB"
  printf 'posts=%s\n' "$POSTS"
  printf 'comments=%s\n' "$COMMENTS"
  printf 'sqlBytes=%s\n' "$(wc -c <"$WORK/$NAME/data/square.sql.gz" | tr -d ' ')"
  printf 'note=帖子正文在 Postgres；图片 URL 指向 OSS，本包不含 OSS 文件。\n'
} >"$WORK/$NAME/meta/manifest.txt"

cat >"$WORK/$NAME/README.txt" <<EOF
谈信广场迁移包 ${NAME}

本机已包含：
- Postgres 全库（帖子、评论、点赞、浏览、关注、举报、公众号、运营账号）
- 广场 / Postgres 镜像（除非打包时 SKIP_IMAGES=1）
- 环境文件 env/square.env（含密钥，请妥善保管）
- 运营台静态文件 web/admin

新机器一键部署：
  tar -xzf ${NAME}.tar.gz
  cd ${NAME}
  ./restore.sh

需要 Docker 与 docker compose。默认端口 18100。
帖子配图在 OSS（SQUARE_OSS_URL），请保证新机器仍能访问原 OSS，否则图片打不开。
EOF

mkdir -p "$OUT_DIR"
ARCHIVE="${OUT_DIR}/${NAME}.tar.gz"
log "压缩 $ARCHIVE"
COPYFILE_DISABLE=1 tar -C "$WORK" -czf "$ARCHIVE" "$NAME"
log "完成 $ARCHIVE"
log "拷到新机器后：tar -xzf ${NAME}.tar.gz && cd ${NAME} && ./restore.sh"
