#!/bin/bash
# MMCS 快速启动脚本
set -e

echo "=== MMCS 多模型协作系统 ==="

# 启动 PostgreSQL 和 Redis
echo "[1/3] 启动数据库服务..."
docker compose up -d postgres redis
echo "      等待数据库就绪..."
sleep 3

# 运行数据库迁移
echo "[2/3] 执行数据库迁移..."
# go run migrations/...  # 迁移工具按需启用

# 启动桌面应用
echo "[3/3] 启动桌面应用..."
open cmd/desktop/build/bin/mmcs.app

echo ""
echo "=== 启动完成 ==="
echo "桌面应用: cmd/desktop/build/bin/mmcs.app"
echo "API 服务: 可通过 docker compose up api-server 启动 HTTP 模式"
