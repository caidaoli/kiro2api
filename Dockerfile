# 多平台构建 Dockerfile
# 使用官方 Go 镜像作为构建环境
FROM --platform=$BUILDPLATFORM golang:1.25.0-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的工具
RUN apk add --no-cache git ca-certificates

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 设置目标平台变量
ARG TARGETOS
ARG TARGETARCH

# 构建应用程序
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -installsuffix cgo -o kiro2api main.go

# 使用轻量级的 alpine 镜像作为运行环境
FROM alpine:3.19

# 安装 ca-certificates 用于 HTTPS 请求
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

# 暴露默认端口
EXPOSE 8080

# 设置默认命令
CMD ["./kiro2api"]
