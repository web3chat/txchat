TARGETDIR=target
projectVersion=$(shell git describe --abbrev=8 --tags 2>/dev/null || git rev-parse --short=8 HEAD 2>/dev/null || echo unknown)
COMPOSE := $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || echo docker-compose)

.PHONY: help build test check-auth-sync tidy image pg run compose-up compose-down migrate-pack migrate-restore clean strip-appledouble

# 默认读 .env；服务器部署：make compose-up ENV_FILE=.env.prod
ENV_FILE ?= .env

help:
	@printf "Usage: make [command]\n"
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-24s\033[0m %s\n", $$1, $$2}'

build: ## 编译 square 到 target/
	@mkdir -p $(TARGETDIR)
	CGO_ENABLED=0 go build -o $(TARGETDIR)/square ./cmd/square

test: ## 跑单测
	go test ./...

check-auth-sync: ## 与 dtalk 验签拷贝对比（找不到 dtalk 则跳过）
	bash ./scripts/check-auth-sync.sh

tidy: ## go mod tidy
	go mod tidy

image: strip-appledouble build ## 构建镜像 txchat-square:<version>
	docker build -f script/docker/Dockerfile -t txchat-square:$(projectVersion) .
	@echo "image: txchat-square:$(projectVersion)"

pg: ## 未指定 SQUARE_PG_HOST 时拉取镜像并创建/复用 txchat-square-pg
	bash ./scripts/ensure-pg.sh

run: ## 启动 square（未指定 PG 时自动拉起 Docker PostgreSQL）
	go run ./cmd/square

strip-appledouble: ## 删除 macOS 附属文件 ._（外置盘 Docker 构建会因 xattr 失败）
	@find . -name '._*' -delete 2>/dev/null || true

compose-up: strip-appledouble ## 创建网络并启动 postgres + square（可用 ENV_FILE=.env.prod）
	docker network inspect txchat-components >/dev/null 2>&1 || docker network create txchat-components
	@echo "compose: $(COMPOSE)"
	@# 旧版 docker-compose 1.29 重建容器会 KeyError ContainerConfig；先删容器再创建（数据在 volume 里）
	-docker rm -f txchat-square-pg txchat-square >/dev/null 2>&1
	@if [ -f "$(ENV_FILE)" ]; then \
		echo "using env file: $(ENV_FILE)"; \
		COPYFILE_DISABLE=1 $(COMPOSE) --env-file "$(ENV_FILE)" -f docker-compose.yml up -d --build; \
	else \
		echo "env file $(ENV_FILE) not found, using compose defaults"; \
		COPYFILE_DISABLE=1 $(COMPOSE) -f docker-compose.yml up -d --build; \
	fi
	@echo "console: http://127.0.0.1:18100/square/console/"

compose-down: ## 停止本项目 compose
	$(COMPOSE) -f docker-compose.yml down

migrate-pack: ## 打包当前部署（库+镜像+配置）到 dist/square-migrate-*.tar.gz
	bash ./scripts/pack-migrate.sh "$(ENV_FILE)"

migrate-restore: ## 新机器一键恢复：make migrate-restore ARCHIVE=dist/square-migrate-xxx.tar.gz
	@test -n "$(ARCHIVE)" || (echo "用法: make migrate-restore ARCHIVE=dist/square-migrate-xxx.tar.gz" >&2; exit 1)
	bash ./scripts/restore-migrate.sh "$(ARCHIVE)"

clean:
	rm -rf $(TARGETDIR)
