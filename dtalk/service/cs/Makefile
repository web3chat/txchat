# golang1.23 or latest
# 1. make build       — 编译 cs 服务（Linux amd64）
# 2. make tools       — 编译 cs-tools 工具
# 3. make pkg         — 打包为 tar.gz
# 4. make upload      — 编译并上传到远程服务器
# 5. make run         — 本地运行

APP_NAME := cs
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DIR := build
APP := ${BUILD_DIR}/${APP_NAME}_v${VERSION}
TOOLS_APP := ${BUILD_DIR}/${APP_NAME}-tools_v${VERSION}
PKG_NAME := ${APP_NAME}_v${VERSION}
PKG := ${PKG_NAME}.tar.gz

main_path 	= "main"
go_version 	= $(shell go version | awk '{ print $$3 }')
build_time 	= $(shell date "+%Y-%m-%d %H:%M:%S %Z")
git_commit 	= $(shell git rev-parse --short=10 HEAD)
flags		:= -ldflags "-X '${main_path}.goVersion=${go_version}' \
-X '${main_path}.buildTime=${build_time}' \
-X '${main_path}.gitCommit=${git_commit}' \
-X 'google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=warn'"

.PHONY: clean build tools pkg upload run

clean: ## Remove previous build
	@rm -rf ${BUILD_DIR}
	@go clean

build: ## Build the cs server binary (Linux amd64)
	GOOS=linux GOARCH=amd64 GO111MODULE=on GOPROXY=https://goproxy.cn,direct GOSUMDB="sum.golang.google.cn" go build -v $(flags) -o $(APP) cmd/main.go

tools: ## Build the cs-tools binary (Linux amd64)
	GOOS=linux GOARCH=amd64 GO111MODULE=on GOPROXY=https://goproxy.cn,direct GOSUMDB="sum.golang.google.cn" go build -v $(flags) -o $(TOOLS_APP) tools/main.go

all: build tools ## Build both server and tools

pkg: build ## Package server binary with config
	mkdir -p ${PKG_NAME}/bin
	mkdir -p ${PKG_NAME}/etc
	cp ${APP} ${PKG_NAME}/bin/
	cp config/cs.toml ${PKG_NAME}/etc/
	tar zvcf ${PKG} ${PKG_NAME}
	rm -rf ${PKG_NAME}

REMOTE_BIN_PATH := /opt/dtalk/srv/app/bin
upload: build ## Build and upload to remote server
	rsync -r $(APP) 107:$(REMOTE_BIN_PATH)
	ssh 107 "cd $(REMOTE_BIN_PATH) && chmod 777 $(PKG_NAME) && ln -sf $(PKG_NAME) $(APP_NAME) && supervisorctl restart $(APP_NAME)"

run: ## Run locally
	go run cmd/main.go -conf config/cs.toml
