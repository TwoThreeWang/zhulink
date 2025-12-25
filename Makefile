.PHONY: dev build css css-build css-watch run-server setup clean

# 开发模式 (热重载 + CSS 监听)
dev:
	make -j2 css-watch run-server

# 生产构建 (编译 Go + 压缩 CSS)
build:
	@echo "Building production assets..."
	make css-build
	@echo "Building Go binary..."
	go build -ldflags="-s -w" -o zhulink ./cmd/server
	@echo "Build complete! Binary: ./zhulink"

# CSS 监听模式 (开发环境)
css-watch:
	npx tailwindcss -i ./web/assets/input.css -o ./web/static/css/style.css --watch

# CSS 构建模式 (生产环境,压缩)
css-build:
	npx tailwindcss -i ./web/assets/input.css -o ./web/static/css/style.css --minify

# 兼容旧命令
css: css-watch

# 运行开发服务器
run-server:
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "Air not found in PATH. Try 'make setup' or 'go run cmd/server/main.go'"; \
		go run cmd/server/main.go; \
	fi

# 安装开发工具
setup:
	go install github.com/air-verse/air@latest
	npm init -y
	npm install -D tailwindcss

# 清理构建产物
clean:
	rm -f zhulink tmp/main
	@echo "Clean complete!"

