# ZhuLink Docker 部署指南

## 📋 部署方案概述

本项目提供了生产级的 Docker Compose 部署方案,具有以下特性:

- ✅ **多阶段构建**: 优化镜像体积 (最终镜像 < 30MB)
- ✅ **自动化构建**: 自动编译 Go 应用和压缩 CSS
- ✅ **数据持久化**: PostgreSQL 数据自动持久化
- ✅ **健康检查**: 自动监控服务健康状态
- ✅ **安全性**: 非 root 用户运行,最小化权限
- ✅ **开发/生产分离**: 提供独立的开发和生产配置

## 🚀 快速开始 (生产部署)

### 1. 准备环境配置

```bash
# 复制环境变量示例文件
cp .env.production.example .env.production

# 编辑配置文件,修改所有必要的配置项
nano .env.production
```

**必须修改的配置项**:
- `DB_PASSWORD`: 数据库密码 (强密码)
- `SESSION_SECRET`: Session 密钥 (至少 32 位随机字符串)
- `SITE_URL`: 你的网站 URL

**可选配置项**:
- `LLM_TOKEN`: Gemini API 密钥 (用于内容审核和摘要)
- `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET`: Google OAuth 登录

### 2. 启动服务

```bash
# 使用生产配置启动
docker-compose --env-file .env.production up -d

# 查看日志
docker-compose logs -f app

# 查看服务状态
docker-compose ps
```

### 3. 访问应用

打开浏览器访问: `http://your-server-ip:8080`

### 4. 停止服务

```bash
# 停止服务
docker-compose down

# 停止服务并删除数据卷 (谨慎使用!)
docker-compose down -v
```

## 🛠 开发环境 (仅数据库)

如果你想在本地开发,只需要 Docker 运行数据库:

```bash
# 启动开发数据库
docker-compose -f docker-compose.dev.yml up -d

# 本地运行应用
make dev
```

开发数据库配置:
- Host: `localhost`
- Port: `5432`
- User: `postgres`
- Password: `postgres`
- Database: `zhulink`

## 📦 Docker 镜像说明

### 多阶段构建流程

1. **阶段 1 - CSS 构建** (`node:20-alpine`):
   - 安装 Tailwind CSS
   - 编译并压缩 CSS 文件
   - 输出: `web/static/css/style.css` (约 39KB)

2. **阶段 2 - Go 编译** (`golang:1.23-alpine`):
   - 下载 Go 依赖
   - 编译静态链接的二进制文件
   - 使用 `-ldflags="-s -w"` 优化体积
   - 输出: `zhulink` (约 20-25MB)

3. **阶段 3 - 运行镜像** (`alpine:latest`):
   - 仅包含必要的运行时文件
   - 非 root 用户运行
   - 最终镜像大小: < 30MB

### 镜像优化特性

- ✅ 静态链接编译 (`CGO_ENABLED=0`)
- ✅ 去除调试信息 (`-s -w`)
- ✅ 多阶段构建 (不包含构建工具)
- ✅ Alpine Linux 基础镜像 (最小化)
- ✅ 非 root 用户运行 (安全)

## 🔧 常用命令

### 服务管理

```bash
# 启动服务
docker-compose up -d

# 停止服务
docker-compose down

# 重启服务
docker-compose restart

# 查看日志
docker-compose logs -f

# 查看特定服务日志
docker-compose logs -f app
docker-compose logs -f postgres
```

### 数据库管理

```bash
# 进入数据库容器
docker-compose exec postgres psql -U zhulink -d zhulink

# 备份数据库
docker-compose exec postgres pg_dump -U zhulink zhulink > backup.sql

# 恢复数据库
docker-compose exec -T postgres psql -U zhulink zhulink < backup.sql
```

### 应用管理

```bash
# 进入应用容器
docker-compose exec app sh

# 重新构建镜像
docker-compose build --no-cache

# 更新应用 (拉取新代码后)
git pull
docker-compose build
docker-compose up -d
```

## 🔐 生产环境最佳实践

### 1. 安全配置

- ✅ 使用强随机密码 (`DB_PASSWORD`, `SESSION_SECRET`)
- ✅ 不要将 `.env.production` 提交到 Git
- ✅ 定期更新 Docker 镜像
- ✅ 配置防火墙,仅开放必要端口

### 2. 反向代理 (推荐使用 Nginx)

```nginx
server {
    listen 80;
    server_name your-domain.com;

    # 重定向到 HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name your-domain.com;

    # SSL 证书配置
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    # 反向代理到 ZhuLink
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # 静态资源缓存
    location /static/ {
        proxy_pass http://localhost:8080;
        proxy_cache_valid 200 30d;
        add_header Cache-Control "public, immutable";
    }
}
```

### 3. 数据备份

建议设置定时备份任务:

```bash
# 创建备份脚本 backup.sh
#!/bin/bash
BACKUP_DIR="/path/to/backups"
DATE=$(date +%Y%m%d_%H%M%S)
docker-compose exec -T postgres pg_dump -U zhulink zhulink | gzip > "$BACKUP_DIR/zhulink_$DATE.sql.gz"

# 删除 30 天前的备份
find "$BACKUP_DIR" -name "zhulink_*.sql.gz" -mtime +30 -delete
```

添加到 crontab:
```bash
# 每天凌晨 3 点备份
0 3 * * * /path/to/backup.sh
```

### 4. 监控和日志

```bash
# 限制日志大小 (在 docker-compose.yml 中添加)
services:
  app:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

### 5. 资源限制

```yaml
# 在 docker-compose.yml 中添加资源限制
services:
  app:
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 512M
        reservations:
          cpus: '0.5'
          memory: 256M
```

## 🐛 故障排查

### 应用无法启动

```bash
# 查看详细日志
docker-compose logs app

# 检查健康状态
docker-compose ps

# 进入容器调试
docker-compose exec app sh
```

### 数据库连接失败

```bash
# 检查数据库是否就绪
docker-compose exec postgres pg_isready -U zhulink

# 查看数据库日志
docker-compose logs postgres

# 测试连接
docker-compose exec app wget -O- http://localhost:8080/
```

### 端口冲突

如果 8080 端口被占用,修改 `.env.production`:
```bash
APP_PORT=8081
```

## 📊 性能优化

### 1. 数据库连接池

应用已配置合理的超时时间,如需调整可在 `cmd/server/main.go` 中修改。

### 2. 静态资源

建议使用 CDN 加速静态资源,或配置 Nginx 缓存。

### 3. 数据库索引

应用已自动创建必要的索引,如有性能问题可查看慢查询日志。

## 🔄 更新部署

```bash
# 1. 拉取最新代码
git pull

# 2. 重新构建镜像
docker-compose build

# 3. 重启服务 (零停机时间)
docker-compose up -d

# 4. 清理旧镜像
docker image prune -f
```

## 📝 环境变量说明

| 变量名 | 必需 | 默认值 | 说明 |
|--------|------|--------|------|
| `DB_NAME` | 否 | `zhulink` | 数据库名称 |
| `DB_USER` | 否 | `zhulink` | 数据库用户 |
| `DB_PASSWORD` | **是** | - | 数据库密码 |
| `DB_PORT` | 否 | `5432` | 数据库端口 |
| `APP_PORT` | 否 | `8080` | 应用端口 |
| `SESSION_SECRET` | **是** | - | Session 密钥 |
| `SITE_URL` | 否 | `http://localhost:8080` | 网站 URL |
| `LLM_BASE_URL` | 否 | - | LLM API 地址 |
| `LLM_MODEL` | 否 | - | LLM 模型名称 |
| `LLM_TOKEN` | 否 | - | LLM API 密钥 |
| `GOOGLE_CLIENT_ID` | 否 | - | Google OAuth ID |
| `GOOGLE_CLIENT_SECRET` | 否 | - | Google OAuth Secret |

## 🆘 获取帮助

如遇到问题:
1. 查看日志: `docker-compose logs -f`
2. 检查配置: 确认 `.env.production` 配置正确
3. 提交 Issue: 附上错误日志和环境信息
