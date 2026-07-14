#!/usr/bin/env bash
# 交叉编译 Go 后端并部署到服务器（systemd 服务 blog-stats）
set -euo pipefail

SERVER="root@159.195.18.74"

cd "$(dirname "$0")/../server"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o blog-stats .

ssh "$SERVER" "mkdir -p /opt/blog-stats"
scp blog-stats "$SERVER:/opt/blog-stats/blog-stats.new"
scp ../deploy/blog-stats.service "$SERVER:/etc/systemd/system/blog-stats.service"
ssh "$SERVER" '
  set -e
  mv /opt/blog-stats/blog-stats.new /opt/blog-stats/blog-stats
  chmod +x /opt/blog-stats/blog-stats
  if [ ! -f /opt/blog-stats/env ]; then
    echo "⚠️  缺少 /opt/blog-stats/env，请参考 deploy/env.example 创建后再启动"
    exit 1
  fi
  systemctl daemon-reload
  systemctl enable --now blog-stats
  systemctl restart blog-stats
  sleep 1
  systemctl is-active blog-stats
  curl -sf http://127.0.0.1:8080/api/healthz && echo "✅ blog-stats 运行正常"
'
rm -f blog-stats
