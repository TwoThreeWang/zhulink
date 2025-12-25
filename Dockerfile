# 多阶段构建 Dockerfile - 生产优化版本

# ============================================
# 阶段 1: 构建前端资源 (Tailwind CSS)
# ============================================
FROM node:20-alpine AS css-builder

WORKDIR /build

# 只复制必要的前端构建文件
COPY package*.json ./
COPY tailwind.config.js ./
COPY web/assets ./web/assets
COPY web/templates ./web/templates

# 安装依赖并构建压缩后的 CSS
RUN npm install && \
    npx tailwindcss -i ./web/assets/input.css -o ./web/static/css/style.css --minify

# ============================================
# 阶段 2: 构建 Go 应用
# ============================================
FROM golang:1.23-alpine AS go-builder

WORKDIR /build

# 安装构建依赖
RUN apk add --no-cache git

# 复制 go mod 文件并下载依赖 (利用 Docker 缓存)
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY cmd ./cmd
COPY internal ./internal

# 编译 Go 应用 (静态链接,优化体积)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -extldflags '-static'" \
    -o zhulink ./cmd/server

# ============================================
# 阶段 3: 最终运行镜像
# ============================================
FROM alpine:latest

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

# 从构建阶段复制编译好的二进制文件
COPY --from=go-builder /build/zhulink .

# 从 CSS 构建阶段复制静态资源
COPY --from=css-builder /build/web/static ./web/static

# 复制模板文件
COPY web/templates ./web/templates

# 创建非 root 用户运行应用
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

# 启动应用
CMD ["./zhulink"]
