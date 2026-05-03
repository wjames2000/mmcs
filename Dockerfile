# =============================================================================
# MMCS — 多模型协作系统
# 多阶段构建 Dockerfile
# =============================================================================

# Stage 1: Build
FROM golang:1.25-alpine AS builder

RUN apk --no-cache add git ca-certificates

WORKDIR /app

# 先复制 go.mod 和 go.sum 以利用 Docker 层缓存
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建二进制文件
# -ldflags="-s -w": 去掉调试信息和符号表，减小二进制体积
# CGO_ENABLED=0: 完全静态编译，不依赖 libc
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/api-server ./cmd/api-server && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/agent-worker ./cmd/agent-worker && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/sse-server ./cmd/sse-server

# Stage 2: Runtime
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata && \
    update-ca-certificates

# 创建非 root 用户
RUN adduser -D -u 1001 appuser

# 复制二进制文件
COPY --from=builder /app/bin/ /usr/local/bin/

# 复制配置文件
COPY --from=builder /app/config/ /config/

# 复制数据库迁移脚本
COPY --from=builder /app/migrations/ /migrations/

# 创建日志目录
RUN mkdir -p /var/log/mmcs && chown -R appuser:appuser /var/log/mmcs

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/healthz || exit 1

CMD ["api-server"]
