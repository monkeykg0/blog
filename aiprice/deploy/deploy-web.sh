#!/usr/bin/env bash
# 把 web/ 源码推到服务器，装依赖，然后触发一次发布。
#
# ⚠️ 日常不需要跑这个。价格每天由 systemd 自动发布：
#     aiprice-crawl.timer → crawl.service →(OnSuccess)→ publish.service
# 只有改了页面模板、样式或依赖之后才跑，把新源码推上去并立即重建一次。
#
# 手动补发一次当天数据（不改代码）：
#     ssh root@159.195.18.74 systemctl start aiprice-publish.service
set -euo pipefail

SERVER="root@159.195.18.74"
APP_DIR=/opt/aiprice
WEB_ROOT="${WEB_ROOT:-/opt/1panel/apps/openresty/openresty/www/sites/aiprice.monkeykgai.com/index}"

cd "$(dirname "$0")/../web"

echo "→ 同步 web 源码"
# src/data 不传：数据由服务器上的 aiprice export 现产，本地那份只是构建缓存。
rsync -az --delete \
  --exclude node_modules --exclude dist --exclude .astro --exclude src/data \
  ./ "$SERVER:$APP_DIR/web/"

echo "→ 同步发布脚本与 systemd 单元"
scp -q ../deploy/publish.sh            "$SERVER:$APP_DIR/publish.sh"
scp -q ../deploy/aiprice-publish.service "$SERVER:/etc/systemd/system/aiprice-publish.service"
scp -q ../deploy/aiprice-crawl.service   "$SERVER:/etc/systemd/system/aiprice-crawl.service"

ssh "$SERVER" "
  set -e
  chmod +x $APP_DIR/publish.sh
  cd $APP_DIR/web && npm ci --no-audit --no-fund
  chown -R www-data:www-data $APP_DIR
  # 站点目录原本是 rsync 从 Mac 传上来的 uid 501，得让 www-data 能写
  chown -R www-data:www-data '$WEB_ROOT'
  systemctl daemon-reload
"

echo "→ 触发一次发布"
ssh "$SERVER" "systemctl start aiprice-publish.service"
ssh "$SERVER" "journalctl -u aiprice-publish -n 25 --no-pager --output=cat"
