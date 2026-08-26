#!/usr/bin/env bash
# 交叉编译 aiprice 抓取程序并部署到服务器（systemd timer 每日触发）
set -euo pipefail

SERVER="root@159.195.18.74"

cd "$(dirname "$0")/../api"
go test ./...
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o aiprice .

ssh "$SERVER" "mkdir -p /opt/aiprice"
scp aiprice "$SERVER:/opt/aiprice/aiprice.new"
scp ../deploy/aiprice-crawl.service "$SERVER:/etc/systemd/system/aiprice-crawl.service"
scp ../deploy/aiprice-crawl.timer "$SERVER:/etc/systemd/system/aiprice-crawl.timer"
ssh "$SERVER" '
  set -e
  mv /opt/aiprice/aiprice.new /opt/aiprice/aiprice
  chmod +x /opt/aiprice/aiprice
  if [ ! -f /opt/aiprice/env ]; then
    echo "⚠️  缺少 /opt/aiprice/env，请参考 deploy/env.example 创建后再启用定时器"
    exit 1
  fi
  chown -R www-data:www-data /opt/aiprice
  systemctl daemon-reload
  systemctl enable --now aiprice-crawl.timer
  echo "--- 自检 ---"
  sudo -u www-data /opt/aiprice/aiprice check
  echo "--- 下次抓取时间 ---"
  systemctl list-timers aiprice-crawl.timer --no-pager | head -3
'
rm -f aiprice
echo "✅ 部署完成。手动跑一次：ssh $SERVER systemctl start aiprice-crawl.service"
echo "   看日志：       ssh $SERVER journalctl -u aiprice-crawl -f"
