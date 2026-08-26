#!/usr/bin/env bash
# 从服务器导出最新价格数据 → 构建 Astro 静态站 → rsync 到 aiprice 站点目录
#
# 数据在服务器的 MySQL 里，所以构建前先让服务器上的 aiprice 导出一份 JSON 拉到本地。
# 传 --skip-export 可跳过这一步，用本地已有的 src/data 构建（改样式时用得上）。
set -euo pipefail

SERVER="root@159.195.18.74"
WEB_ROOT="${WEB_ROOT:-/opt/1panel/apps/openresty/openresty/www/sites/aiprice.monkeykgai.com/index}"
REMOTE_TMP="/tmp/aiprice-export"

cd "$(dirname "$0")/../web"

if [ "${1:-}" != "--skip-export" ]; then
  echo "→ 在服务器上导出最新数据"
  ssh "$SERVER" "
    set -euo pipefail
    set -a; . /opt/aiprice/env; set +a
    rm -rf $REMOTE_TMP && mkdir -p $REMOTE_TMP
    /opt/aiprice/aiprice export -out $REMOTE_TMP
  "
  rm -rf src/data && mkdir -p src/data
  scp -q -r "$SERVER:$REMOTE_TMP/." src/data/
  echo "→ 数据日期: $(node -p "require('./src/data/meta.json').dataDate")"
fi

npm run build

# --delete 会清掉服务器上多余的文件，但 1Panel 建站时放的 404.html / index.html
# 会被我们的构建产物覆盖掉，这是预期的。
rsync -avz --delete dist/ "$SERVER:$WEB_ROOT/"

echo "✅ 已部署到 https://aiprice.monkeykgai.com"
echo "   HTML 边缘缓存 1 小时，要立刻看到新价格就去 Cloudflare 清一下缓存"
