GO ?= go
AIR ?= air
BINARY_NAME := pxe-server
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

.PHONY: all build run test clean dev help
.PHONY: linux linux-arm windows darwin darwin-arm
.PHONY: build-all install lint fmt test-coverage

# 默认目标
all: build

# 当前平台编译
build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) ./cmd/server

# Linux amd64 交叉编译（产物命名与 deploy.sh BIN_NAME / CI 一致）
# CGO_ENABLED=0：纯 Go SQLite（modernc），静态无 glibc 依赖，无需交叉 C 编译器
linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME)-linux-amd64 ./cmd/server

# Linux arm64 交叉编译（aarch64 装机端如为 ARM 服务器本机跑）
linux-arm:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME)-linux-arm64 ./cmd/server

# Windows amd64 交叉编译
windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME)-windows-amd64.exe ./cmd/server

# macOS amd64 (Intel) 交叉编译
darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME)-darwin-amd64 ./cmd/server

# macOS arm64 (Apple Silicon) 交叉编译
darwin-arm:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME)-darwin-arm64 ./cmd/server

# 编译所有架构
build-all: linux linux-arm windows darwin darwin-arm
	@echo "所有平台编译完成"
	@ls -lh $(BINARY_NAME)-*

# 运行（使用默认参数）
run: build
	./$(BINARY_NAME) -db data/pxe-server.db -log logs/pxe-server.log -level info

# 开发模式：热加载（修改代码自动重编译重启）
dev:
	$(AIR)

# 运行测试
test:
	$(GO) test ./... -v

# 运行测试（带覆盖率）
test-coverage:
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告已生成: coverage.html"

# 代码检查
lint:
	@which golangci-lint > /dev/null || (echo "请先安装 golangci-lint" && exit 1)
	golangci-lint run

# 格式化代码
fmt:
	$(GO) fmt ./...
	gofmt -s -w .

# 安装到 GOPATH/bin
install:
	$(GO) install -ldflags "$(LDFLAGS)" ./cmd/server

# 清理编译产物
clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME)-*
	rm -rf tmp/ coverage.out coverage.html

# 显示帮助信息
help:
	@echo "PXE Server Makefile"
	@echo ""
	@echo "可用目标:"
	@echo "  build        - 编译当前平台二进制"
	@echo "  linux        - 编译 Linux amd64"
	@echo "  linux-arm    - 编译 Linux arm64"
	@echo "  windows      - 编译 Windows amd64"
	@echo "  darwin       - 编译 macOS amd64 (Intel)"
	@echo "  darwin-arm   - 编译 macOS arm64 (Apple Silicon)"
	@echo "  build-all    - 编译所有平台"
	@echo "  run          - 编译并运行"
	@echo "  dev          - 开发模式（热加载）"
	@echo "  test         - 运行测试"
	@echo "  test-coverage- 运行测试并生成覆盖率报告"
	@echo "  lint         - 代码检查（需要 golangci-lint）"
	@echo "  fmt          - 格式化代码"
	@echo "  install      - 安装到 GOPATH/bin"
	@echo "  clean        - 清理编译产物"
	@echo "  help         - 显示此帮助信息"
	@echo ""
	@echo "变量:"
	@echo "  VERSION=$(VERSION)"
	@echo "  BUILD_TIME=$(BUILD_TIME)"

