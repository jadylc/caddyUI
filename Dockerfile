# syntax=docker/dockerfile:1.7

# ===== 阶段1：编译前端 =====
FROM --platform=$BUILDPLATFORM node:24.14.0-alpine3.23 AS ui-builder
WORKDIR /build
COPY ui/package*.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY ui/ ./
RUN npm run build

# ===== 阶段2：编译带插件的 Caddy =====
FROM --platform=$BUILDPLATFORM caddy:builder AS caddy-builder
ARG TARGETOS
ARG TARGETARCH
ARG GOPROXY=http://goproxy.cn,direct
ARG HTTP_PROXY=""
ARG HTTPS_PROXY=""
ARG NO_PROXY="*"
ENV GOPROXY=$GOPROXY HTTP_PROXY="" HTTPS_PROXY="" NO_PROXY="*"
# Patch dnspod provider: support delegated zones and stable libdns return records
COPY patches/dnspod-patched.go /tmp/dnspod-patched.go
RUN --mount=type=cache,target=/go/pkg/mod \
    set -e \
    && mkdir -p /tmp/dnspod-src && cd /tmp/dnspod-src \
    && GO111MODULE=on go mod init temp 2>/dev/null \
    && go get github.com/caddy-dns/dnspod@fb7cc31c \
    && DNSTOD_MOD="$(go env GOMODCACHE)/github.com/caddy-dns/dnspod@v0.0.5-0.20260325061251-fb7cc31cc04c" \
    && cp /tmp/dnspod-patched.go "$DNSTOD_MOD/dnspod.go" \
    && cd /tmp && rm -rf /tmp/dnspod-src \
    && echo "=== patched dnspod provider ==="
ENV GONOSUMCHECK=github.com/caddy-dns/dnspod GONOSUMDB=github.com/caddy-dns/dnspod
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=$TARGETOS GOARCH=$TARGETARCH xcaddy build --output /usr/bin/caddy \
    --with github.com/caddy-dns/alidns \
    --with github.com/caddy-dns/cloudflare \
    --with github.com/caddy-dns/dnspod=github.com/caddy-dns/dnspod@fb7cc31c \
    --with github.com/caddy-dns/he \
    --with github.com/mholt/caddy-l4 \
    --with github.com/mholt/caddy-dynamicdns \
    --with golang.org/x/net/publicsuffix@latest \
    && echo "=== built caddy deps ===" \
    && go version -m /usr/bin/caddy | grep -E 'golang.org/x/net|dnspod' || true

# ===== 阶段3：编译 site 管理 API（Go 静态二进制） =====
FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS api-builder
ARG TARGETOS
ARG TARGETARCH
ARG GOPROXY=http://goproxy.cn,direct
# 清除 Docker Desktop 注入的失效代理，避免 Go 模块下载 TLS 超时
ENV GOPROXY=$GOPROXY HTTP_PROXY="" HTTPS_PROXY="" http_proxy="" https_proxy=""
WORKDIR /build
COPY backend/go.mod ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY backend/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /caddy-api .

# ===== 阶段4：最终运行镜像（裸 alpine，避免继承 caddy:alpine 的 EXPOSE） =====
FROM alpine:3.20
RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories \
    && apk add --no-cache ca-certificates mailcap tzdata \
    && mkdir -p /data/caddy /config/caddy /config/sites /etc/caddy/global.d /srv/ui
# 定制版 Caddy（含阿里云、Cloudflare、DNSPod 与 Dynamic DNS 插件）
COPY --from=caddy-builder /usr/bin/caddy /usr/bin/caddy
# 前端构建产物
COPY --from=ui-builder /build/dist /srv/ui
# site 管理 API
COPY --from=api-builder /caddy-api /usr/bin/caddy-api
# Caddy 配置和启动脚本
COPY Caddyfile /etc/caddy/Caddyfile
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Caddy 默认存储位置
ENV XDG_CONFIG_HOME=/config XDG_DATA_HOME=/data TZ=Asia/Shanghai

EXPOSE 8888
ENTRYPOINT ["/entrypoint.sh"]
