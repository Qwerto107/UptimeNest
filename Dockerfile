# 本地源码构建前后端；CI 镜像直接复用已打包的内嵌程序。
FROM --platform=$BUILDPLATFORM node:24-alpine AS frontend
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build
WORKDIR /src
COPY scripts/ ./scripts/
RUN node scripts/prepare-webui.mjs

FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine AS backend
WORKDIR /src
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY --from=frontend /src/internal/webui/dist ./internal/webui/dist
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
RUN case "${TARGETARCH}/${TARGETVARIANT}" in \
      arm/v5) export GOARM=5 ;; \
      arm/v6) export GOARM=6 ;; \
      arm/*) echo "不支持的 ARM 变体" >&2; exit 1 ;; \
    esac \
    && GOOS=$TARGETOS GOARCH=$TARGETARCH go build -tags=with_frontend -trimpath -ldflags="-s -w" -o /out/uptimenest ./cmd/uptimenest

# 在构建主机准备通用文件，发布镜像无需执行目标架构程序。
FROM --platform=$BUILDPLATFORM alpine:3.23 AS runtime-files
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 65532 uptimenest \
    && adduser -D -H -u 65532 -G uptimenest uptimenest \
    && mkdir -p /app/data \
    && chown 65532:65532 /app/data

# Ping 依赖系统命令，因此保留 Alpine 运行层及对应架构的 BusyBox。
FROM alpine:3.23 AS runtime-base

FROM runtime-base AS runtime
COPY --from=runtime-files /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=runtime-files /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=runtime-files /etc/passwd /etc/passwd
COPY --from=runtime-files /etc/group /etc/group
COPY --from=runtime-files --chown=65532:65532 /app/data /app/data
# 专用 ping 入口以系统权限打开 ICMP 套接字，主程序仍以普通用户运行。
COPY --from=runtime-base --chmod=4755 /bin/busybox /usr/local/bin/ping
ENV UPTIMENEST_ADDRESS=0.0.0.0:8090 \
    UPTIMENEST_DB_DSN=/app/data/uptimenest.db
USER 65532:65532
WORKDIR /app
EXPOSE 8090
VOLUME ["/app/data"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD ["/app/uptimenest", "healthcheck"]
ENTRYPOINT ["/app/uptimenest"]

FROM --platform=$BUILDPLATFORM alpine:3.23 AS release-binary
ARG TARGETARCH
ARG TARGETVARIANT
COPY .release/binaries/ /binaries/
RUN platform="${TARGETARCH}" \
    && if [ "$TARGETARCH" = arm ]; then platform="arm${TARGETVARIANT}"; fi \
    && cp "/binaries/uptimenest-linux-${platform}/uptimenest" /uptimenest \
    && chmod 0755 /uptimenest

FROM runtime AS release
COPY --from=release-binary /uptimenest /app/uptimenest

# 默认目标保留 docker compose --build 的源码构建方式。
FROM runtime AS standalone
COPY --from=backend /out/uptimenest /app/uptimenest
