.PHONY: all build run clean test deps install uninstall docker dev fmt lint help

# 项目配置
PROJECT := lingguard
CMD_DIR := cmd/lingguard

# 构建信息
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# 安装配置
PREFIX ?= $(HOME)/.local

# 默认目标
all: build

# 下载依赖
deps:
	go mod download
	go mod tidy

# 构建 - 输出到当前目录
build:
	go build $(LDFLAGS) -o $(PROJECT) ./$(CMD_DIR)

# 构建并运行
run: build
	./$(PROJECT)

# 清理
clean:
	go clean
	rm -f $(PROJECT)

# 测试
test:
	go test -v ./...

# 安装到系统（包括 systemd 服务和配置）
install: build
	@echo "安装 LingGuard..."
	PREFIX=$(PREFIX) bash scripts/install.sh
	systemctl --user daemon-reload
	systemctl --user restart lingguard.service

# 卸载
uninstall:
	@echo "卸载 LingGuard..."
	PREFIX=$(PREFIX) bash scripts/uninstall.sh

# 仅安装二进制文件（不含配置和服务）
install-bin: build
	install -m 755 $(PROJECT) $(PREFIX)/bin/

# 打包发布
package: build
	@echo "打包发布版本..."
	mkdir -p dist
	# Linux amd64
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(PROJECT)-linux-amd64 ./$(CMD_DIR)
	tar -czf dist/$(PROJECT)-$(VERSION)-linux-amd64.tar.gz -C dist $(PROJECT)-linux-amd64 --transform 's|.*|lingguard/|'
	# Linux arm64
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(PROJECT)-linux-arm64 ./$(CMD_DIR)
	tar -czf dist/$(PROJECT)-$(VERSION)-linux-arm64.tar.gz -C dist $(PROJECT)-linux-arm64 --transform 's|.*|lingguard/|'
	# 包含配置和脚本
	cp -r configs dist/
	cp -r scripts dist/
	cp -r skills dist/
	tar -rf dist/$(PROJECT)-$(VERSION)-linux-amd64.tar.gz -C dist configs scripts skills --transform 's|dist/||'
	gzip -f dist/$(PROJECT)-$(VERSION)-linux-amd64.tar
	@echo "发布包已创建: dist/$(PROJECT)-$(VERSION)-linux-amd64.tar.gz"

# Docker 构建
docker:
	docker build -t $(PROJECT):$(VERSION) .

# 开发模式（直接运行，不构建）
dev:
	go run ./$(CMD_DIR)

# 格式化代码
fmt:
	go fmt ./...

# 静态检查
lint:
	golangci-lint run

# 帮助
help:
	@echo "LingGuard - 个人 AI 助手"
	@echo ""
	@echo "构建命令:"
	@echo "  make build      - 构建项目（输出到当前目录）"
	@echo "  make run        - 构建并运行"
	@echo "  make clean      - 清理构建产物"
	@echo "  make test       - 运行测试"
	@echo "  make deps       - 下载依赖"
	@echo ""
	@echo "安装命令:"
	@echo "  make install    - 完整安装（二进制 + 配置 + systemd 服务）"
	@echo "  make install-bin - 仅安装二进制文件到 $(PREFIX)/bin"
	@echo "  make uninstall  - 卸载"
	@echo "  make package    - 打包发布版本"
	@echo ""
	@echo "开发命令:"
	@echo "  make dev        - 开发模式运行"
	@echo "  make fmt        - 格式化代码"
	@echo "  make lint       - 静态检查"
	@echo "  make docker     - 构建 Docker 镜像"
	@echo ""
	@echo "安装变量:"
	@echo "  PREFIX=/usr/local make install  - 指定安装前缀"
