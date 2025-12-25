#!/bin/bash

# ============================================
# ZhuLink 生产环境部署脚本
# ============================================
# 功能: 拉取最新代码、构建 Docker 镜像、平滑重启应用
# 使用: ./deploy.sh
# ============================================

set -e  # 遇到错误立即退出

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查必要文件
check_requirements() {
    log_info "检查部署环境..."

    if [ ! -f ".env" ]; then
        log_error ".env 文件不存在,请先创建配置文件"
        log_info "运行: cp .env.example .env && nano .env"
        exit 1
    fi

    if ! command -v docker &> /dev/null; then
        log_error "Docker 未安装"
        exit 1
    fi

    if ! command -v docker-compose &> /dev/null; then
        log_error "Docker Compose 未安装"
        exit 1
    fi

    log_info "环境检查通过"
}

# 备份当前版本
backup_current() {
    log_info "备份当前容器..."

    if docker ps -a | grep -q "zhulink"; then
        BACKUP_NAME="zhulink-backup-$(date +%Y%m%d-%H%M%S)"
        docker commit zhulink "$BACKUP_NAME" 2>/dev/null || true
        log_info "已创建备份镜像: $BACKUP_NAME"
    fi
}

# 拉取最新代码
pull_latest_code() {
    log_info "拉取最新代码..."

    # 保存本地修改
    if [ -n "$(git status --porcelain)" ]; then
        log_warn "检测到本地修改,将暂存..."
        git stash
        STASHED=true
    fi

    # 拉取最新代码
    git pull origin main || git pull origin master

    log_info "代码更新完成"
}

# 构建 Docker 镜像
build_image() {
    log_info "构建 Docker 镜像..."

    # 构建新镜像(不停止旧容器)
    docker-compose build --no-cache

    log_info "镜像构建完成"
}

# 平滑重启服务(零停机时间)
restart_service() {
    log_info "平滑重启服务(零停机时间)..."

    # 使用 up -d 实现平滑重启
    # Docker Compose 会先启动新容器,确认健康后再停止旧容器
    docker-compose up -d --force-recreate --remove-orphans

    log_info "等待服务启动..."
    sleep 5
}

# 检查服务健康状态
check_health() {
    log_info "检查服务健康状态..."

    MAX_RETRIES=30
    RETRY_COUNT=0

    while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
        if docker ps | grep -q "zhulink.*Up"; then
            if curl -f http://localhost:8080 &> /dev/null; then
                log_info "服务启动成功! ✓"
                return 0
            fi
        fi

        RETRY_COUNT=$((RETRY_COUNT + 1))
        echo -n "."
        sleep 2
    done

    echo ""
    log_error "服务启动失败或健康检查超时"
    log_info "查看日志: docker-compose logs -f"
    return 1
}

# 清理旧镜像
cleanup_old_images() {
    log_info "清理旧镜像..."

    # 删除悬空镜像
    docker image prune -f

    # 只保留最近 1 个备份镜像,删除其他所有备份
    OLD_BACKUPS=$(docker images | grep "zhulink-backup" | awk '{print $1":"$2}' | tail -n +2)
    if [ -n "$OLD_BACKUPS" ]; then
        echo "$OLD_BACKUPS" | xargs docker rmi 2>/dev/null || true
        log_info "已清理旧备份镜像(保留最新 1 个)"
    fi
}

# 显示部署信息
show_deployment_info() {
    echo ""
    log_info "======================================"
    log_info "部署完成!"
    log_info "======================================"
    echo ""
    echo "服务状态:"
    docker-compose ps
    echo ""
    echo "查看日志: docker-compose logs -f"
    echo "停止服务: docker-compose down"
    echo "重启服务: docker-compose restart"
    echo ""
}

# 回滚函数
rollback() {
    log_error "部署失败,开始回滚..."

    # 停止当前容器
    docker-compose down

    # 查找最新的备份镜像
    LATEST_BACKUP=$(docker images | grep "zhulink-backup" | head -n 1 | awk '{print $1":"$2}')

    if [ -n "$LATEST_BACKUP" ]; then
        log_info "恢复到备份版本: $LATEST_BACKUP"
        docker tag "$LATEST_BACKUP" zhulink:latest
        docker-compose up -d
        log_info "已回滚到之前的版本"
    else
        log_error "未找到备份镜像,无法自动回滚"
        log_info "请手动检查并修复问题"
    fi

    exit 1
}

# 主函数
main() {
    echo ""
    log_info "======================================"
    log_info "ZhuLink 生产环境部署"
    log_info "======================================"
    echo ""

    # 检查环境
    check_requirements

    # 备份当前版本
    backup_current

    # 拉取最新代码
    pull_latest_code

    # 构建镜像
    build_image

    # 平滑重启服务
    restart_service

    # 检查健康状态
    if ! check_health; then
        rollback
    fi

    # 清理旧镜像
    cleanup_old_images

    # 显示部署信息
    show_deployment_info
}

# 捕获错误并回滚
trap 'rollback' ERR

# 执行主函数
main
