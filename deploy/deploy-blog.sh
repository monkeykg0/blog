#!/usr/bin/env bash
# 构建 Astro 博客并 rsync 到服务器
set -euo pipefail

SERVER="root@159.195.18.74"
WEB_ROOT="${WEB_ROOT:-/opt/1panel/apps/openresty/openresty/www/sites/monkeykgai.com/index}"

cd "$(dirname "$0")/../blog"
pnpm run build
rsync -avz --delete dist/ "$SERVER:$WEB_ROOT/"
echo "✅ 博客已部署"
