GO ?= go
AIR ?= air

.PHONY: build run test clean linux linux-arm dev

# 当前平台编译
build:
	$(GO) build -o pxe-server ./cmd/server

# Linux amd64 交叉编译
linux:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(GO) build -o pxe-server-linux ./cmd/server

# Linux arm64 交叉编译（aarch64 装机端如为 ARM 服务器本机跑）
linux-arm:
	CGO_ENABLED=1 GOOS=linux GOARCH=arm64 $(GO) build -o pxe-server-linux-arm64 ./cmd/server

run: build
	./pxe-server -db data/pxe-server.db -log logs/pxe-server.log -level info

# 开发模式：热加载（修改代码自动重编译重启）
dev:
	$(AIR)

test:
	$(GO) test ./... -v

clean:
	rm -f pxe-server pxe-server-linux pxe-server-linux-arm64
	rm -rf tmp/
