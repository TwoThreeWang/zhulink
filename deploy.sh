#!/bin/sh
# 拉取最新代码并重建 ZhuLink。用法：sh deploy.sh
set -eu

cd "$(dirname "$0")"

if [ ! -f .env ]; then
    echo "未找到 .env，请先根据 .env.example 创建生产配置。" >&2
    exit 1
fi

echo "正在拉取最新代码..."
git pull --ff-only

# --wait：等容器健康检查通过再返回，启动失败时脚本以非 0 退出
echo "正在重建并启动..."
docker compose up -d --build --force-recreate --remove-orphans --wait

docker image prune -f
docker compose ps
