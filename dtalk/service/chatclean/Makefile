# chatclean proto 生成：与 auth 保持一致（参考 service/auth/api/auth.pb.go）
# 版本要求：
#   protoc-gen-go     v1.27.1
#   protoc            v3.19.1（或 3.19.x）
#   protoc-gen-go-grpc v1.2.0（与项目 grpc v1.40 兼容）
#
# 安装生成器到本地项目：make install-tools

API_DIR := api
PROTO_FILE := $(API_DIR)/chatclean.proto
# 模块根目录（dtalk，含 go.mod 与 cmd/main.go）；在 service/chatclean 下执行 make 时 ROOT=../..
ROOT := .
# 生成器安装到项目 bin 目录，gen-proto 会优先使用
TOOLS_BIN := $(ROOT)/bin
APP := chatclean_v0.1.0
OUTPUT := $(ROOT)/build/$(APP)
# Dockerfile / docker-compose 默认从 deploy/chatclean 复制二进制
DEPLOY_BIN := $(ROOT)/deploy/chatclean
# 与模块根 go.mod 一致：GOTOOLCHAIN=auto 表示使用 go.mod 中 toolchain 指定的 Go 版本
export GOTOOLCHAIN := auto

# 可选：版本信息，可通过 make build 或 make build LDFLAGS= 覆盖
GO_VERSION := $(shell go version 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date '+%Y-%m-%d %H:%M:%S %Z' 2>/dev/null || echo "unknown")
GIT_COMMIT := $(shell git -C $(ROOT) rev-parse --short=9 HEAD 2>/dev/null || echo "unknown")
LDFLAGS ?= -X 'main.goVersion=$(GO_VERSION)' -X 'main.buildTime=$(BUILD_TIME)' -X 'main.gitCommit=$(GIT_COMMIT)' -X 'google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=warn'

.PHONY: install-tools gen-proto build
install-tools:
	@mkdir -p $(TOOLS_BIN)
	cd $(ROOT) && GOBIN=$(abspath $(TOOLS_BIN)) go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.27.1
	cd $(ROOT) && GOBIN=$(abspath $(TOOLS_BIN)) go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2.0
	@echo "生成器已安装到 $(TOOLS_BIN)，gen-proto 将自动使用该目录"

gen-proto:
	PATH="$(abspath $(TOOLS_BIN)):$$PATH" protoc -I $(API_DIR) \
		--go_out=$(API_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(API_DIR) --go-grpc_opt=paths=source_relative \
		$(PROTO_FILE)
	@echo "chatclean api 已生成（与 auth 同版本：protoc-gen-go v1.27.1, protoc v3.19.1, protoc-gen-go-grpc v1.2.0）"

build:
	@mkdir -p $(ROOT)/build $(ROOT)/deploy
	@echo "使用 Go 版本（与 go.mod toolchain 一致）: $$(cd $(ROOT) && go version)"
	GOOS=linux GOARCH=amd64 GO111MODULE=on GOPROXY=https://goproxy.cn,direct GOSUMDB=sum.golang.google.cn go build -v -ldflags "$(LDFLAGS)" -o $(OUTPUT) ./cmd/main.go
	cp -f $(OUTPUT) $(DEPLOY_BIN)
	chmod +x $(DEPLOY_BIN)
	@echo "已输出: $(OUTPUT)"
	@echo "已复制到部署目录: $(DEPLOY_BIN)"

clean:
	find . -name '._*' -delete