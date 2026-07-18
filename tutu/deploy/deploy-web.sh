#!/usr/bin/env bash
# 构建 React 前端并 rsync 到服务器
# token 从服务器 /opt/media-api/env 读取(单一事实来源),需先跑过 deploy-api.sh
set -euo pipefail
export LC_ALL=en_US.UTF-8 LANG=en_US.UTF-8

SERVER="root@159.195.18.74"
WEB_ROOT="/opt/1panel/apps/openresty/openresty/www/media/web"

TOKEN="$(ssh "$SERVER" "grep '^MEDIA_TOKEN=' /opt/media-api/env | cut -d= -f2")"
[ -n "$TOKEN" ] || { echo "❌ 服务器上没有 MEDIA_TOKEN,先跑 deploy-api.sh"; exit 1; }

cd "$(dirname "$0")/../web"
VITE_MEDIA_TOKEN="$TOKEN" pnpm run build

ssh "$SERVER" "mkdir -p '$WEB_ROOT'"
rsync -avz --delete dist/ "$SERVER:$WEB_ROOT/"
echo "✅ 前端已部署 → https://monkeykgai.com/tutu/"
