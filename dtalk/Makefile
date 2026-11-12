# golang1.17 or latest
TARGETDIR=target
# 本地工具目录，make install-swag 会安装 swag 到此
BIN := bin

# 无 tag 时 git describe 会失败（私有仓常未推送 tags），回退到短 commit，避免镜像 tag 变成 txchat-xxx:
projectVersion=$(shell git describe --abbrev=8 --tags 2>/dev/null || git rev-parse --short=8 HEAD)
gitCommit=$(shell git rev-parse --short=8 HEAD)
# 再兜底，防止极端情况下仍为空
ifeq ($(strip $(projectVersion)),)
projectVersion := unknown
endif

pkgCommitName=${projectVersion}_${gitCommit}
# make update MODULES="pusher gateway" 时使用；也可逗号分隔
MODULES ?=

# 可选预置：script/compose/key（与 MYSQL 密码同源，见 key.example）
# 命令行 make VAR=... > key > 下方默认值
KEY_FILE := script/compose/key
ifneq (,$(wildcard $(KEY_FILE)))
  _key_get = $(shell grep -E '^$(1)=' $(KEY_FILE) 2>/dev/null | head -1 | cut -d= -f2- | tr -d '\r')
  _KEY_FEDERATION_ENABLED := $(call _key_get,FEDERATION_ENABLED)
  _KEY_CLUSTER_ID := $(call _key_get,CLUSTER_ID)
  _KEY_PRESENCE_URL := $(call _key_get,PRESENCE_URL)
  _KEY_GENERATOR_NODE := $(call _key_get,GENERATOR_NODE)
  ifneq ($(strip $(_KEY_FEDERATION_ENABLED)),)
    FEDERATION_ENABLED ?= $(_KEY_FEDERATION_ENABLED)
  endif
  ifneq ($(strip $(_KEY_CLUSTER_ID)),)
    CLUSTER_ID ?= $(_KEY_CLUSTER_ID)
  endif
  ifneq ($(strip $(_KEY_PRESENCE_URL)),)
    PRESENCE_URL ?= $(_KEY_PRESENCE_URL)
  endif
  ifneq ($(strip $(_KEY_GENERATOR_NODE)),)
    GENERATOR_NODE ?= $(_KEY_GENERATOR_NODE)
  endif
endif

# 联邦全局变量（init-compose / apply-federation 写入各模块 toml，并写入 run_compose/.env）
CLUSTER_ID ?= cluster-a
PRESENCE_URL ?= http://txchat-presence:18050
# default：联邦 init-compose 不使用这个 1，改为向 Presence 随机占 128～1023
# cli / environment / key：当作指定节点，占坑失败不悄悄换号
GENERATOR_NODE_SOURCE := default
ifeq ($(origin GENERATOR_NODE),command line)
  GENERATOR_NODE_SOURCE := cli
else ifeq ($(origin GENERATOR_NODE),environment)
  GENERATOR_NODE_SOURCE := cli
else ifneq ($(strip $(_KEY_GENERATOR_NODE)),)
  GENERATOR_NODE_SOURCE := key
endif
GENERATOR_NODE ?= 1
# default：联邦 init-compose 不使用 cluster-a，改为 cluster-<节点号>-<8位随机串>
CLUSTER_ID_SOURCE := default
ifeq ($(origin CLUSTER_ID),command line)
  CLUSTER_ID_SOURCE := cli
else ifeq ($(origin CLUSTER_ID),environment)
  CLUSTER_ID_SOURCE := cli
else ifneq ($(strip $(_KEY_CLUSTER_ID)),)
  CLUSTER_ID_SOURCE := key
endif
# 默认单集群：不编译/不启动 federation，不依赖 Presence
FEDERATION_ENABLED ?= false

base_servers=answer backend backup call device discovery gateway generator group offline-push oss pusher store chatclean chatadmin
servers=$(base_servers)
ifeq ($(FEDERATION_ENABLED),true)
servers += federation
endif

help: ## Display this help screen
	@printf "Help doc:\nUsage: make [command]\n"
	@printf "[command]\n"
	@grep -h -E '^([a-zA-Z_-]|\%)+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: clean ## 编译本机系统和指令集的可执行文件
	./script/builder/builder.sh ${TARGETDIR} "" "${servers}"

build_%: clean ## 编译目标机器的可执行文件（例如: make build_linux_amd64）
	./script/builder/builder.sh ${TARGETDIR} $* "${servers}"

pkg: build ## 编译并打包本机系统和指令集的可执行文件
	tar -zcvf ${TARGETDIR}_'host'_${pkgCommitName}.tar.gz ${TARGETDIR}/

pkg_%: build_% ## 编译并打包目标机器的可执行文件（例如: make pkg_linux_amd64）
	tar -zcvf ${TARGETDIR}_$*_${pkgCommitName}.tar.gz ${TARGETDIR}/

images: build_linux_amd64 ## 打包docker镜像
	cp script/docker/*Dockerfile ${TARGETDIR}
	bash ./script/compose/build-images.sh ${TARGETDIR} ${projectVersion} "$(servers)"

init-compose: ## 编译镜像并初始化 compose（可读 script/compose/key；或 FEDERATION_ENABLED=true CLUSTER_ID=...）
	@test -f script/compose/init-compose.sh || (echo "ERROR: 缺少 script/compose/init-compose.sh，请先 git pull" >&2; exit 1)
	@command -v bash >/dev/null 2>&1 || (echo "ERROR: 未找到 bash（Error 127）" >&2; exit 1)
	@if [ -f script/compose/key ]; then echo "info: 已加载预置 script/compose/key"; fi
	FEDERATION_ENABLED="$(FEDERATION_ENABLED)" CLUSTER_ID="$(CLUSTER_ID)" CLUSTER_ID_SOURCE="$(CLUSTER_ID_SOURCE)" PRESENCE_URL="$(PRESENCE_URL)" GENERATOR_NODE="$(GENERATOR_NODE)" GENERATOR_NODE_SOURCE="$(GENERATOR_NODE_SOURCE)" \
		bash ./script/compose/init-compose.sh "$(projectVersion)" "$(gitCommit)" "$(servers)"

apply-federation: ## 按 .env/变量把 CLUSTER_ID 等写入 target/*.toml。用法: make apply-federation FEDERATION_ENABLED=true CLUSTER_ID=cluster-b
	@test -f script/compose/apply-federation-config.sh || (echo "ERROR: 缺少 apply-federation-config.sh" >&2; exit 1)
	@cid="$(CLUSTER_ID)"; purl="$(PRESENCE_URL)"; gnode="$(GENERATOR_NODE)"; feden="$(FEDERATION_ENABLED)"; \
	if [ "$(origin CLUSTER_ID)" != "command line" ] && [ -f run_compose/.env ]; then \
	  v=$$(grep -E '^CLUSTER_ID=' run_compose/.env | head -1 | cut -d= -f2-); [ -n "$$v" ] && cid="$$v"; \
	fi; \
	if [ "$(origin PRESENCE_URL)" != "command line" ] && [ -f run_compose/.env ]; then \
	  v=$$(grep -E '^PRESENCE_URL=' run_compose/.env | head -1 | cut -d= -f2-); [ -n "$$v" ] && purl="$$v"; \
	fi; \
	if [ "$(origin GENERATOR_NODE)" != "command line" ] && [ -f run_compose/.env ]; then \
	  v=$$(grep -E '^GENERATOR_NODE=' run_compose/.env | head -1 | cut -d= -f2-); [ -n "$$v" ] && gnode="$$v"; \
	fi; \
	if [ "$(origin FEDERATION_ENABLED)" != "command line" ] && [ -f run_compose/.env ]; then \
	  v=$$(grep -E '^FEDERATION_ENABLED=' run_compose/.env | head -1 | cut -d= -f2-); [ -n "$$v" ] && feden="$$v"; \
	fi; \
	FEDERATION_ENABLED="$$feden" CLUSTER_ID="$$cid" PRESENCE_URL="$$purl" GENERATOR_NODE="$$gnode" \
		bash ./script/compose/apply-federation-config.sh
check-federation: ## 校验 federation/pusher/gateway/generator/logic 的 clusterId 与 presenceUrl 是否一致
	@test -f script/compose/check-federation-config.sh || (echo "ERROR: 缺少 check-federation-config.sh" >&2; exit 1)
	@if [ "$(origin CLUSTER_ID)" = "command line" ]; then export CLUSTER_ID="$(CLUSTER_ID)"; fi; \
	if [ "$(origin PRESENCE_URL)" = "command line" ]; then export PRESENCE_URL="$(PRESENCE_URL)"; fi; \
	if [ "$(origin GENERATOR_NODE)" = "command line" ]; then export GENERATOR_NODE="$(GENERATOR_NODE)"; fi; \
	bash ./script/compose/check-federation-config.sh

migrate-federation-mysql: ## 幂等补建/升级 dtalk_record 联邦表（已有 MySQL 数据卷时必跑）
	@test -f script/compose/migrate-federation-mysql.sh || (echo "ERROR: 缺少 migrate-federation-mysql.sh" >&2; exit 1)
	@command -v bash >/dev/null 2>&1 || (echo "ERROR: 未找到 bash" >&2; exit 1)
	bash ./script/compose/migrate-federation-mysql.sh

update: ## 热更新指定模块（不改 .env）。用法: make update MODULES="federation" FEDERATION_ENABLED=true
	@test -f script/compose/update.sh || (echo "ERROR: 缺少 script/compose/update.sh" >&2; exit 1)
	@command -v bash >/dev/null 2>&1 || (echo "ERROR: 未找到 bash" >&2; exit 1)
	@mods="$(MODULES)"; \
	if [ -z "$$mods" ]; then \
	  echo "ERROR: 请指定 MODULES，例如: make update MODULES=\"pusher gateway\"" >&2; \
	  echo "支持: $(servers)" >&2; \
	  exit 1; \
	fi; \
	mods=$$(echo "$$mods" | tr ',' ' '); \
	feden="$(FEDERATION_ENABLED)"; \
	if [ "$(origin FEDERATION_ENABLED)" != "command line" ] && [ -f run_compose/.env ]; then \
	  v=$$(grep -E '^FEDERATION_ENABLED=' run_compose/.env | head -1 | cut -d= -f2- | tr -d '\r'); \
	  [ -n "$$v" ] && feden="$$v"; \
	fi; \
	FEDERATION_ENABLED="$$feden" bash ./script/compose/update.sh $$mods

docker-compose-up: ## 使用 docker compose 启动（自动清理 update 残留的残缺 compose 标签）
	@test -f script/compose/compose-up.sh || (echo "ERROR: 缺少 script/compose/compose-up.sh，请 git pull" >&2; exit 1)
	bash ./script/compose/compose-up.sh

docker-compose-%: ## 使用docker compose 命令(服务列表：make docker-compose-ls；停止服务：make docker-compose-stop；卸载服务：make docker-compose-down)
	@if [ ! -d "run_compose/" ]; then \
       cp -R script/compose/. run_compose/; \
     fi; \
	@COMPOSE=$$(command -v docker-compose >/dev/null 2>&1 && echo docker-compose || echo "docker compose"); \
    cd run_compose && \
    $$COMPOSE -f components.compose.yaml -f service.compose.yaml $*

destroy: ## 销毁本项目部署（仅 dtalk 容器/卷/自建镜像与 run_compose；不动 im/presence/共享网络）
	@test -f script/compose/reset.sh || (echo "ERROR: 缺少 script/compose/reset.sh" >&2; exit 1)
	bash ./script/compose/reset.sh -y

reset: destroy ## 同 destroy
reset-compose: destroy ## 兼容旧命令，等同于 make destroy

.PHONY: doc install-swag swag test destroy reset reset-compose update apply-federation check-federation
doc:
	./script/doc/doc.sh v1

install-swag: ## 安装 swag 到项目 bin 目录（解决 make swag 报 No such file or directory）
	@mkdir -p $(BIN)
	GOBIN=$(abspath $(BIN)) go install github.com/swaggo/swag/cmd/swag@latest
	@echo "swag 已安装到 $(BIN)/swag，执行 make swag 将自动使用"

swag: ## 根据注释生成 Swagger 文档（需先 make install-swag 或本机已安装 swag）
	@echo '┌ start gen swag'
	@PATH="$(abspath $(BIN)):$$PATH" swag init -g internal/handler/routes.go
	@echo '└ end gen swag'

test: ## 运行 service 目录下各微服务的测试（go test ./service/...）
	$(GOENV) go test -v ./service/...

clean:
	rm -rf ${TARGETDIR}