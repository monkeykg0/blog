#!/usr/bin/env bash
# 交叉编译 media-api 并部署为 systemd 服务
# 首次运行自动生成 /opt/media-api/env(token 随机,MySQL 密码复用 blog-stats)
set -euo pipefail
export LC_ALL=en_US.UTF-8 LANG=en_US.UTF-8

SERVER="root@159.195.18.74"
MEDIA_ROOT="/opt/1panel/apps/openresty/openresty/www/media/library"

cd "$(dirname "$0")/../api"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o media-api .

ssh "$SERVER" "mkdir -p /opt/media-api '$MEDIA_ROOT'"
scp media-api "$SERVER:/opt/media-api/media-api.new"
scp ../deploy/media-api.service "$SERVER:/etc/systemd/system/media-api.service"
rm -f media-api

ssh "$SERVER" "
  set -e
  # 首次:自动生成 env
  if [ ! -f /opt/media-api/env ]; then
    DSN=\$(grep '^MYSQL_DSN=' /opt/blog-stats/env)
    cat > /opt/media-api/env <<EOF
LISTEN=127.0.0.1:8081
MEDIA_ROOT=$MEDIA_ROOT
MEDIA_TOKEN=\$(openssl rand -hex 16)
\$DSN
EOF
    chmod 600 /opt/media-api/env
    echo '已生成 /opt/media-api/env'
  fi
  mv /opt/media-api/media-api.new /opt/media-api/media-api
  chmod +x /opt/media-api/media-api
  systemctl daemon-reload
  systemctl enable --now media-api
  systemctl restart media-api
  sleep 1
  systemctl is-active media-api
  curl -sf http://127.0.0.1:8081/api/media/healthz && echo ' ✅ media-api 运行正常'
"
