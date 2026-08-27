#!/usr/bin/env bash
# 服务器端发布：导出最新数据 → 构建 Astro 静态站 → 同步到站点目录。
#
# 由 aiprice-publish.service 调用。抓取成功后 systemd 自动触发（crawl.service 的
# OnSuccess=），平时不用手动跑。要手动补发一次：
#   systemctl start aiprice-publish.service
set -euo pipefail

APP_DIR=/opt/aiprice
WEB_ROOT="${WEB_ROOT:-/opt/1panel/apps/openresty/openresty/www/sites/aiprice.monkeykgai.com/index}"

cd "$APP_DIR/web"

echo "→ 导出数据"
rm -rf src/data && mkdir -p src/data
"$APP_DIR/aiprice" export -out src/data
echo "  数据日期 $(node -p "require('./src/data/meta.json').dataDate")" \
     "· 快照 $(node -p "require('./src/data/meta.json').days") 天" \
     "· 价格 $(node -p "require('./src/data/meta.json').priceCount") 条"

echo "→ 构建静态站"
npm run build

# --delete 清掉上一版多余的地区页（地区表增删时会有）。
echo "→ 同步到 $WEB_ROOT"
rsync -a --delete dist/ "$WEB_ROOT/"

echo "✅ 发布完成 https://aiprice.monkeykgai.com"
echo "   HTML 在 Cloudflare 边缘缓存 1 小时，要立刻看到新价格就去清缓存"
