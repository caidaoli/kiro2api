# 多平台构建 Dockerfile
# 使用交叉编译架构，支持高效的 arm64 和 amd64 构建
# 技术方案：tonistiigi/xx + Clang/LLVM 交叉编译

# 启用 BuildKit 新特性
# syntax=docker/dockerfile:1.4

# 构建阶段 - 使用 BUILDPLATFORM 在原生架构执行
FROM --platform=$BUILDPLATFORM golang:alpine AS builder

# 安装交叉编译工具链
# tonistiigi/xx 提供跨架构编译辅助工具
COPY --from=tonistiigi/xx:1.6.1 / /
RUN apk add --no-cache git ca-certificates tzdata clang lld

# 设置工作目录
WORKDIR /app

# 配置目标平台的交叉编译工具链
ARG TARGETPLATFORM
RUN xx-apk add musl-dev gcc

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖（在原生平台执行，速度快）
RUN --mount=type=cache,target=/root/.cache/go-mod \
    go mod download

# 复制源代码
COPY . .

# 交叉编译二进制文件（启用 CGO 以支持 bytedance/sonic）
# xx-go 自动设置 GOOS/GOARCH/CC 等环境变量
ENV CGO_ENABLED=1
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/.cache/go-mod \
    xx-go build \
    -ldflags="-s -w" \
    -o kiro2api main.go && \
    xx-verify kiro2api

# 运行阶段
FROM alpine:3.19

# 安装运行时依赖
RUN apk --no-cache add ca-certificates tzdata

# 创建非 root 用户
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件和静态资源
COPY --from=builder /app/kiro2api .
COPY --from=builder /app/static ./static

# 创建必要的目录并设置权限
RUN mkdir -p /home/appuser/.aws/sso/cache && \
    chown -R appuser:appgroup /app /home/appuser

# 切换到非 root 用户
USER appuser

ARG PORT=8080

ENV PORT=$PORT

# 暴露默认端口
EXPOSE $PORT

# 设置默认命令
CMD ["./kiro2api"]
