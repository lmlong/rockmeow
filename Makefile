.PHONY: all build run clean test deps install uninstall docker dev fmt lint help
.PHONY: package package-all package-linux package-darwin package-windows
.PHONY: package-linux-amd64 package-linux-arm64
.PHONY: package-darwin-amd64 package-darwin-arm64
.PHONY: package-windows-amd64 package-windows-arm64

# 项目配置
PROJECT := lingguard
CMD_DIR := cmd/lingguard

# 构建信息
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date '+%Y%m%d%H%M%S')
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# 安装配置
PREFIX ?= $(HOME)/.local

# 默认目标
all: build

# 下载依赖
deps:
	go mod download
	go mod tidy

# 构建 - 输出到当前目录（当前平台）
build:
	go build $(LDFLAGS) -o $(PROJECT) ./$(CMD_DIR)

# 构建并运行
run: build
	./$(PROJECT)

# 清理
clean:
	go clean
	rm -f $(PROJECT)
	rm -f $(PROJECT)-linux $(PROJECT)-darwin $(PROJECT).exe
	rm -rf dist

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

# ============================================================
# 打包发布 - 全平台
# ============================================================

# 打包所有平台
package-all:
	@rm -rf dist
	@$(MAKE) package-linux package-darwin package-windows
	@echo ""
	@echo "============================================"
	@echo "所有平台打包完成！"
	@echo "============================================"
	@ls -lh dist/

# 默认打包 Linux + macOS
package:
	@rm -rf dist
	@$(MAKE) package-linux package-darwin
	@echo ""
	@echo "============================================"
	@echo "打包完成！"
	@echo "============================================"
	@ls -lh dist/

# ============================================================
# Linux 打包
# ============================================================
package-linux: package-linux-amd64 package-linux-arm64

package-linux-amd64:
	@echo "打包 Linux amd64..."
	@mkdir -p dist/$(PROJECT)-linux-amd64
	cp -r configs dist/$(PROJECT)-linux-amd64/
	cp -r scripts dist/$(PROJECT)-linux-amd64/
	cp -r skills dist/$(PROJECT)-linux-amd64/
	@# 移除代理设置（打包发布时不包含本地代理配置）
	@sed -i '/HTTP_PROXY\|HTTPS_PROXY\|NO_PROXY/d' dist/$(PROJECT)-linux-amd64/scripts/lingguard.service
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(PROJECT)-linux-amd64/$(PROJECT) ./$(CMD_DIR)
	cd dist && tar -czf $(PROJECT)-linux-amd64-$(BUILD_TIME).tar.gz $(PROJECT)-linux-amd64
	@echo "已创建: dist/$(PROJECT)-linux-amd64-$(BUILD_TIME).tar.gz"
	@rm -rf dist/$(PROJECT)-linux-amd64

package-linux-arm64:
	@echo "打包 Linux arm64..."
	@mkdir -p dist/$(PROJECT)-linux-arm64
	cp -r configs dist/$(PROJECT)-linux-arm64/
	cp -r scripts dist/$(PROJECT)-linux-arm64/
	cp -r skills dist/$(PROJECT)-linux-arm64/
	@# 移除代理设置（打包发布时不包含本地代理配置）
	@sed -i '/HTTP_PROXY\|HTTPS_PROXY\|NO_PROXY/d' dist/$(PROJECT)-linux-arm64/scripts/lingguard.service
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(PROJECT)-linux-arm64/$(PROJECT) ./$(CMD_DIR)
	cd dist && tar -czf $(PROJECT)-linux-arm64-$(BUILD_TIME).tar.gz $(PROJECT)-linux-arm64
	@echo "已创建: dist/$(PROJECT)-linux-arm64-$(BUILD_TIME).tar.gz"
	@rm -rf dist/$(PROJECT)-linux-arm64

# ============================================================
# macOS 打包
# ============================================================
package-darwin: package-darwin-amd64 package-darwin-arm64

package-darwin-amd64:
	@echo "打包 macOS amd64 (Intel Mac)..."
	@mkdir -p dist/$(PROJECT)-darwin-amd64
	cp -r configs dist/$(PROJECT)-darwin-amd64/
	cp -r scripts dist/$(PROJECT)-darwin-amd64/
	cp -r skills dist/$(PROJECT)-darwin-amd64/
	@# 移除代理设置（打包发布时不包含本地代理配置）
	@sed -i '/HTTP_PROXY\|HTTPS_PROXY\|NO_PROXY/d' dist/$(PROJECT)-darwin-amd64/scripts/lingguard.service
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(PROJECT)-darwin-amd64/$(PROJECT) ./$(CMD_DIR)
	cd dist && tar -czf $(PROJECT)-darwin-amd64-$(BUILD_TIME).tar.gz $(PROJECT)-darwin-amd64
	@echo "已创建: dist/$(PROJECT)-darwin-amd64-$(BUILD_TIME).tar.gz"
	@rm -rf dist/$(PROJECT)-darwin-amd64

package-darwin-arm64:
	@echo "打包 macOS arm64 (Apple Silicon M1/M2/M3)..."
	@mkdir -p dist/$(PROJECT)-darwin-arm64
	cp -r configs dist/$(PROJECT)-darwin-arm64/
	cp -r scripts dist/$(PROJECT)-darwin-arm64/
	cp -r skills dist/$(PROJECT)-darwin-arm64/
	@# 移除代理设置（打包发布时不包含本地代理配置）
	@sed -i '/HTTP_PROXY\|HTTPS_PROXY\|NO_PROXY/d' dist/$(PROJECT)-darwin-arm64/scripts/lingguard.service
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(PROJECT)-darwin-arm64/$(PROJECT) ./$(CMD_DIR)
	cd dist && tar -czf $(PROJECT)-darwin-arm64-$(BUILD_TIME).tar.gz $(PROJECT)-darwin-arm64
	@echo "已创建: dist/$(PROJECT)-darwin-arm64-$(BUILD_TIME).tar.gz"
	@rm -rf dist/$(PROJECT)-darwin-arm64

# ============================================================
# Windows 打包
# ============================================================
package-windows: package-windows-amd64 package-windows-arm64

package-windows-amd64:
	@echo "打包 Windows amd64..."
	@mkdir -p dist/$(PROJECT)-windows-amd64
	cp -r configs dist/$(PROJECT)-windows-amd64/
	cp -r scripts dist/$(PROJECT)-windows-amd64/
	cp -r skills dist/$(PROJECT)-windows-amd64/
	@# 移除代理设置（打包发布时不包含本地代理配置）
	@sed -i '/HTTP_PROXY\|HTTPS_PROXY\|NO_PROXY/d' dist/$(PROJECT)-windows-amd64/scripts/lingguard.service
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(PROJECT)-windows-amd64/$(PROJECT).exe ./$(CMD_DIR)
	cd dist && zip -r $(PROJECT)-windows-amd64-$(BUILD_TIME).zip $(PROJECT)-windows-amd64
	@echo "已创建: dist/$(PROJECT)-windows-amd64-$(BUILD_TIME).zip"
	@rm -rf dist/$(PROJECT)-windows-amd64

package-windows-arm64:
	@echo "打包 Windows arm64..."
	@mkdir -p dist/$(PROJECT)-windows-arm64
	cp -r configs dist/$(PROJECT)-windows-arm64/
	cp -r scripts dist/$(PROJECT)-windows-arm64/
	cp -r skills dist/$(PROJECT)-windows-arm64/
	@# 移除代理设置（打包发布时不包含本地代理配置）
	@sed -i '/HTTP_PROXY\|HTTPS_PROXY\|NO_PROXY/d' dist/$(PROJECT)-windows-arm64/scripts/lingguard.service
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o dist/$(PROJECT)-windows-arm64/$(PROJECT).exe ./$(CMD_DIR)
	cd dist && zip -r $(PROJECT)-windows-arm64-$(BUILD_TIME).zip $(PROJECT)-windows-arm64
	@echo "已创建: dist/$(PROJECT)-windows-arm64-$(BUILD_TIME).zip"
	@rm -rf dist/$(PROJECT)-windows-arm64

# ============================================================
# Docker 构建
# ============================================================
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
	@echo "  make build              - 构建项目（输出到当前目录）"
	@echo "  make run                - 构建并运行"
	@echo "  make clean              - 清理构建产物"
	@echo "  make test               - 运行测试"
	@echo "  make deps               - 下载依赖"
	@echo ""
	@echo "打包命令:"
	@echo "  make package            - 打包 Linux + macOS"
	@echo "  make package-all        - 打包所有平台 (Linux + macOS + Windows)"
	@echo "  make package-linux      - 打包 Linux (amd64 + arm64)"
	@echo "  make package-darwin     - 打包 macOS (Intel + Apple Silicon)"
	@echo "  make package-windows    - 打包 Windows (amd64 + arm64)"
	@echo ""
	@echo "  单独打包:"
	@echo "  make package-linux-amd64   - Linux x86_64"
	@echo "  make package-linux-arm64   - Linux ARM64"
	@echo "  make package-darwin-amd64  - macOS Intel"
	@echo "  make package-darwin-arm64  - macOS Apple Silicon (M1/M2/M3)"
	@echo "  make package-windows-amd64 - Windows x86_64"
	@echo "  make package-windows-arm64 - Windows ARM64"
	@echo ""
	@echo "安装命令:"
	@echo "  make install            - 完整安装（二进制 + 配置 + systemd 服务）"
	@echo "  make install-bin        - 仅安装二进制文件到 $(PREFIX)/bin"
	@echo "  make uninstall          - 卸载"
	@echo ""
	@echo "开发命令:"
	@echo "  make dev                - 开发模式运行"
	@echo "  make fmt                - 格式化代码"
	@echo "  make lint               - 静态检查"
	@echo "  make docker             - 构建 Docker 镜像"
	@echo ""
	@echo "安装变量:"
	@echo "  PREFIX=/usr/local make install  - 指定安装前缀"
