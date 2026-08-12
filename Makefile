GO ?= go
AIR ?= air

.PHONY: build run test clean linux linux-arm dev

# 当前平台编译
build:
	$(GO) build -o pxe-server ./cmd/server

# Linux amd64 交叉编译（产物命名与 deploy.sh BIN_NAME / CI 一致）
# CGO_ENABLED=0：纯 Go SQLite（modernc），静态无 glibc 依赖，无需交叉 C 编译器
linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -o pxe-server-linux-amd64 ./cmd/server

# Linux arm64 交叉编译（aarch64 装机端如为 ARM 服务器本机跑）
linux-arm:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -o pxe-server-linux-arm64 ./cmd/server

run: build
	./pxe-server -db data/pxe-server.db -log logs/pxe-server.log -level info

# 开发模式：热加载（修改代码自动重编译重启）
dev:
	$(AIR)

test:
	$(GO) test ./... -v

clean:
	rm -f pxe-server pxe-server-linux-amd64 pxe-server-linux-arm64
	rm -rf tmp/
