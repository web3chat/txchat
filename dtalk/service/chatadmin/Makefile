# 群组管理服务 Makefile

# 项目信息
PROJECT_NAME = chatadmin
VERSION = 1.0.0
BUILD_TIME = $(shell date +%Y-%m-%d_%H:%M:%S)
GIT_COMMIT = $(shell git rev-parse --short HEAD)
GO_VERSION = $(shell go version | awk '{print $$3}')

# 构建参数
LDFLAGS = -X main.projectVersion=$(VERSION) \
          -X main.goVersion=$(GO_VERSION) \
          -X main.gitCommit=$(GIT_COMMIT) \
          -X main.buildTime=$(BUILD_TIME) \
          -X main.osArch=$(shell go env GOOS)/$(shell go env GOARCH)

# 目录
BUILD_DIR = build
BIN_DIR = $(BUILD_DIR)/bin
CONFIG_DIR = $(BUILD_DIR)/config

# 默认目标
.PHONY: all
all: build

# 构建二进制文件
.PHONY: build
build: clean
	@echo "Building $(PROJECT_NAME)..."
	@mkdir -p $(BIN_DIR)
	@mkdir -p $(CONFIG_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(PROJECT_NAME) ./cmd/main.go
	@cp config/chatadmin.toml $(CONFIG_DIR)/
	@echo "Build completed: $(BIN_DIR)/$(PROJECT_NAME)"

# 快速构建（仅编译二进制）
.PHONY: quick_build
quick_build:
	@echo "Quick building $(PROJECT_NAME)..."
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(PROJECT_NAME) ./cmd/main.go
	@echo "Quick build completed: $(BIN_DIR)/$(PROJECT_NAME)"
	
.PHONY: build_linux
build_linux: clean
	@echo "Building $(PROJECT_NAME)..."
	@mkdir -p $(BIN_DIR)
	@mkdir -p $(CONFIG_DIR)
	# Linux amd64
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(PROJECT_NAME)-linux-amd64 ./cmd/main.go
	
# 构建所有架构
.PHONY: build_all
build_all: clean
	@echo "Building $(PROJECT_NAME) for all architectures..."
	@mkdir -p $(BIN_DIR)
	@mkdir -p $(CONFIG_DIR)
	# Linux amd64
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(PROJECT_NAME)-linux-amd64 ./cmd/main.go
	# Linux arm64
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(PROJECT_NAME)-linux-arm64 ./cmd/main.go
	# Windows amd64
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(PROJECT_NAME)-windows-amd64.exe ./cmd/main.go
	# Darwin amd64
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(PROJECT_NAME)-darwin-amd64 ./cmd/main.go
	# Darwin arm64
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(PROJECT_NAME)-darwin-arm64 ./cmd/main.go
	@cp config/chatadmin.toml $(CONFIG_DIR)/
	@echo "Build all architectures completed"

# 运行服务
.PHONY: run
run: build
	@echo "Running $(PROJECT_NAME)..."
	cd $(BUILD_DIR) && ./bin/$(PROJECT_NAME) -conf=config/chatadmin.toml

# 开发模式运行
.PHONY: dev
dev:
	@echo "Running $(PROJECT_NAME) in development mode..."
	go run ./cmd/main.go -conf=config/chatadmin.toml

# 测试
.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...

# 测试覆盖率
.PHONY: test_coverage
test_coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# 代码格式化
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

# 代码检查
.PHONY: lint
lint:
	@echo "Running linter..."
	golangci-lint run

# 生成swagger文档
.PHONY: swag
swag:
	@echo "Generating swagger documentation..."
	swag init -g ./cmd/main.go -o ./docs

# 清理构建文件
.PHONY: clean
clean:
	@echo "Cleaning build files..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

# 安装依赖
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download

# 显示版本信息
.PHONY: version
version:
	@echo "Project: $(PROJECT_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Go Version: $(GO_VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "OS/Arch: $(shell go env GOOS)/$(shell go env GOARCH)"

# 帮助信息
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  quick_build  - Quick build (binary only)"
	@echo "  build_all    - Build for all architectures"
	@echo "  run          - Build and run the service"
	@echo "  dev          - Run in development mode"
	@echo "  test         - Run tests"
	@echo "  test_coverage- Run tests with coverage"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linter"
	@echo "  swag         - Generate swagger docs"
	@echo "  clean        - Clean build files"
	@echo "  deps         - Install dependencies"
	@echo "  version      - Show version info"
	@echo "  help         - Show this help"
