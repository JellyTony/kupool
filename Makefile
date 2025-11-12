# 项目名称
PROJECT_NAME := kupool

# 版本信息
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date +%Y-%m-%dT%H:%M:%S%z)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 编译变量
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# 目录
BIN_DIR := bin
CLIENT_DIR := cmd/kupool-client
SERVER_DIR := cmd/kupool-server

# 平台和架构
PLATFORMS := darwin/amd64 darwin/arm64 windows/amd64 windows/386 linux/amd64 linux/arm64

# 临时目录
TEMP_DIR := $(shell mktemp -d)

# 默认目标
.PHONY: all
all: clean build

# 创建bin目录
$(BIN_DIR):
	mkdir -p $(BIN_DIR)

# 编译所有平台
.PHONY: build
build: build-client build-server

# 编译客户端
.PHONY: build-client
build-client: $(BIN_DIR)
	@echo "Building client for multiple platforms..."
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		output_name=$(PROJECT_NAME)-client-$$os-$$arch; \
		if [ $$os = "windows" ]; then output_name=$$output_name.exe; fi; \
		echo "Building $$output_name..."; \
		GOOS=$$os GOARCH=$$arch go build $(LDFLAGS) -o $(BIN_DIR)/$$output_name ./$(CLIENT_DIR); \
	done

# 编译服务端
.PHONY: build-server
build-server: $(BIN_DIR)
	@echo "Building server for multiple platforms..."
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		output_name=$(PROJECT_NAME)-server-$$os-$$arch; \
		if [ $$os = "windows" ]; then output_name=$$output_name.exe; fi; \
		echo "Building $$output_name..."; \
		GOOS=$$os GOARCH=$$arch go build $(LDFLAGS) -o $(BIN_DIR)/$$output_name ./$(SERVER_DIR); \
	done

# 编译macOS版本
.PHONY: build-mac
build-mac: $(BIN_DIR)
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/$(PROJECT_NAME)-client-darwin-amd64 ./$(CLIENT_DIR)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/$(PROJECT_NAME)-server-darwin-amd64 ./$(SERVER_DIR)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BIN_DIR)/$(PROJECT_NAME)-client-darwin-arm64 ./$(CLIENT_DIR)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BIN_DIR)/$(PROJECT_NAME)-server-darwin-arm64 ./$(SERVER_DIR)

# 编译Windows版本
.PHONY: build-win
build-win: $(BIN_DIR)
	@echo "Building for Windows..."
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/$(PROJECT_NAME)-client-windows-amd64.exe ./$(CLIENT_DIR)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/$(PROJECT_NAME)-server-windows-amd64.exe ./$(SERVER_DIR)
	GOOS=windows GOARCH=386 go build $(LDFLAGS) -o $(BIN_DIR)/$(PROJECT_NAME)-client-windows-386.exe ./$(CLIENT_DIR)
	GOOS=windows GOARCH=386 go build $(LDFLAGS) -o $(BIN_DIR)/$(PROJECT_NAME)-server-windows-386.exe ./$(SERVER_DIR)

# 编译当前平台
.PHONY: build-local
build-local: $(BIN_DIR)
	@echo "Building for current platform..."
	go build $(LDFLAGS) -o $(BIN_DIR)/$(PROJECT_NAME)-client ./$(CLIENT_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/$(PROJECT_NAME)-server ./$(SERVER_DIR)

# 清理编译产物
.PHONY: clean
clean:
	@echo "Cleaning up..."
	rm -rf $(BIN_DIR)

# 运行测试
.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...

# 运行测试并生成覆盖率报告
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

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

# 安装依赖
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# 帮助信息
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all          - Clean and build for all platforms"
	@echo "  build        - Build client and server for all platforms"
	@echo "  build-client - Build client for all platforms"
	@echo "  build-server - Build server for all platforms"
	@echo "  build-mac    - Build for macOS (amd64 and arm64)"
	@echo "  build-win    - Build for Windows (amd64 and 386)"
	@echo "  build-local  - Build for current platform"
	@echo "  clean        - Remove build artifacts"
	@echo "  test         - Run tests"
	@echo "  test-coverage- Run tests with coverage report"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linter"
	@echo "  deps         - Install dependencies"
	@echo "  help         - Show this help message"