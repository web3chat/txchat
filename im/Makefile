# golang1.17 or latest
TARGETDIR=target

# 只匹配 v* 根标签：子模块 tag（如 im/v0.1.0）带斜杠，不能用作镜像 tag。
# 无匹配 tag 时回退到短 commit，避免镜像 tag 变成 txchat-xxx:
projectVersion=$(shell git describe --abbrev=8 --tags --match 'v*' 2>/dev/null || git rev-parse --short=8 HEAD)
gitCommit=$(shell git rev-parse --short=8 HEAD)
ifeq ($(strip $(projectVersion)),)
projectVersion := unknown
endif

pkgCommitName=${projectVersion}_${gitCommit}
servers=comet logic

# 可选预置：script/compose/key（见 key.example）；未配置则从 dtalk .env 同步
KEY_FILE := script/compose/key
ifneq (,$(wildcard $(KEY_FILE)))
  _key_get = $(shell grep -E '^$(1)=' $(KEY_FILE) 2>/dev/null | head -1 | cut -d= -f2- | tr -d '\r')
  _KEY_CLUSTER_ID := $(call _key_get,CLUSTER_ID)
  _KEY_PRESENCE_URL := $(call _key_get,PRESENCE_URL)
  ifneq ($(strip $(_KEY_CLUSTER_ID)),)
    CLUSTER_ID ?= $(_KEY_CLUSTER_ID)
  endif
  ifneq ($(strip $(_KEY_PRESENCE_URL)),)
    PRESENCE_URL ?= $(_KEY_PRESENCE_URL)
  endif
endif
CLUSTER_ID ?=
PRESENCE_URL ?=

help: ## 查看makefile帮助文档
	@printf "Help doc:\nUsage: make [command]\n"
	@printf "[command]\n"
	@grep -h -E '^([a-zA-Z_-]|\%)+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: clean ## 编译本机系统和指令集的可执行文件
	./script/build/builder.sh ${TARGETDIR} "" "$(servers)"

build_%: clean ## 编译目标机器的可执行文件（例如: make build_linux_amd64）
	./script/build/builder.sh ${TARGETDIR} $* "$(servers)"

pkg: build ## 编译并打包本机系统和指令集的可执行文件
	tar -zcvf ${TARGETDIR}_'host'_${pkgCommitName}.tar.gz ${TARGETDIR}/

pkg_%: build_% ## 编译并打包目标机器的可执行文件（例如: make pkg_linux_amd64）
	tar -zcvf ${TARGETDIR}_$*_${pkgCommitName}.tar.gz ${TARGETDIR}/

images: build_linux_amd64 ## 打包docker镜像
	cp script/docker/*Dockerfile ${TARGETDIR}
	bash ./script/compose/build-images.sh ${TARGETDIR} $(projectVersion) "$(servers)"

init-compose: ## 编译镜像并初始化 compose（可读 script/compose/key；MYSQL 仍从 dtalk 同步）
	@test -f script/compose/init-compose.sh || (echo "ERROR: 缺少 script/compose/init-compose.sh" >&2; exit 1)
	@command -v bash >/dev/null 2>&1 || (echo "ERROR: 未找到 bash" >&2; exit 1)
	@if [ -f script/compose/key ]; then echo "info: 已加载预置 script/compose/key"; fi
	DTALK_ENV="$(DTALK_ENV)" DTALK_ROOT="$(DTALK_ROOT)" \
	CLUSTER_ID="$(CLUSTER_ID)" PRESENCE_URL="$(PRESENCE_URL)" \
		bash ./script/compose/init-compose.sh "$(projectVersion)" "$(gitCommit)" "$(servers)"

# 使用 docker-compose 独立命令（与 dtalk 一致）。
# 若写成 docker compose -f，在未安装 Compose V2 插件时会变成 docker -f，报 unknown shorthand flag: 'f'
COMPOSE := $(shell command -v docker-compose >/dev/null 2>&1 && echo docker-compose || echo "docker compose")

docker-compose-up:  ## 使用docker compose启动
	@if [ ! -d "run_compose/" ]; then \
		echo "ERROR: run_compose/ 不存在，请先执行 make init-compose" >&2; \
		exit 1; \
	 fi; \
	cd run_compose && \
	$(COMPOSE) -f components.compose.yaml -f service.compose.yaml up -d

docker-compose-%: ## 使用docker compose 命令(服务列表：make docker-compose-ls；停止服务：make docker-compose-stop；卸载服务：make docker-compose-down)
	@if [ ! -d "run_compose/" ]; then \
       cp -R script/compose/. run_compose/; \
     fi; \
    cd run_compose && \
    $(COMPOSE) -f components.compose.yaml -f service.compose.yaml $*

destroy: ## 销毁本项目部署（仅 im 容器/卷/自建镜像与 run_compose；不动 dtalk/presence/共享网络）
	@test -f script/compose/reset.sh || (echo "ERROR: 缺少 script/compose/reset.sh" >&2; exit 1)
	bash ./script/compose/reset.sh -y

reset: destroy ## 同 destroy

test:
	$(GOENV) go test -v ./...

clean:
	rm -rf ${TARGETDIR}

run:
	for i in $(servers) ; do \
		nohup ${TARGETDIR}/$$i -conf=${TARGETDIR}/$$i.toml -logtostderr 2>&1 > ${TARGETDIR}/$$i.log; \
	done

stop:
	for i in $(servers) ; do \
	  	pkill -f ${TARGETDIR}/$$i; \
	done

.PHONY: fmt_proto fmt_shell fmt_go destroy reset

fmt: fmt_proto fmt_shell fmt_go ## format all files

fmt_proto: ## format protobuf file
	@find . -name '*.proto' -not -path "./vendor/*" | xargs clang-format -i

fmt_shell: ## format shell file
	@find . -name '*.sh' -not -path "./vendor/*" | xargs shfmt -w -s -i 4 -ci -bn

fmt_go: ## format go
	@find . -name '*.go' -not -path "./vendor/*" | xargs gofmt -s -w
	@find . -name '*.go' -not -path "./vendor/*" | xargs goimports -l -w

.PHONY: checkgofmt linter linter_test

check: checkgofmt linter ## check format and linter

checkgofmt: ## get all go files and run go fmt on them
	@files=$$(find . -name '*.go' -not -path "./vendor/*" | xargs gofmt -l -s); if [ -n "$$files" ]; then \
		  echo "Error: 'make fmt' needs to be run on:"; \
		  find . -name '*.go' -not -path "./vendor/*" | xargs gofmt -l -s ;\
		  exit 1; \
		  fi;
	@files=$$(find . -name '*.go' -not -path "./vendor/*" | xargs goimports -l -w); if [ -n "$$files" ]; then \
		  echo "Error: 'make fmt' needs to be run on:"; \
		  find . -name '*.go' -not -path "./vendor/*" | xargs goimports -l -w ;\
		  exit 1; \
		  fi;

linter: ## Use gometalinter check code, ignore some unserious warning
	@golangci-lint run ./... && find . -name '*.sh' -not -path "./vendor/*" | xargs shellcheck

linter_test: ## Use gometalinter check code, for local test
	@chmod +x ./script/golinter.sh
	@./script/golinter.sh "test" "${p}"
	@find . -name '*.sh' -not -path "./vendor/*" | xargs shellcheck
