#!/bin/sh
set -e

# 业务路由 / 全局片段目录 / 日志目录
mkdir -p /config/sites /etc/caddy/global.d /config/caddy
touch /config/caddy/caddy.log

# 同步写入文件和 Docker 日志，避免 Caddy 启动失败时只在卷内可见。
tail -n 0 -F /config/caddy/caddy.log &

# 根据已存的凭据生成 /etc/caddy/global.d/acme.conf（若无则确保文件不存在）
caddy-api -sync >> /config/caddy/caddy.log 2>&1 || true

# 启动 caddy-api 把日志打到日志文件
caddy-api >> /config/caddy/caddy.log 2>&1 &

# 前台运行 Caddy，日志打到日志文件
exec caddy run --config /etc/caddy/Caddyfile --adapter caddyfile >> /config/caddy/caddy.log 2>&1
