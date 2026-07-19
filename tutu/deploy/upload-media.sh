#!/usr/bin/env bash
# 上传 media-library 到服务器(断点续传),完成后通知 media-api 刷新专辑
# 用法: deploy/upload-media.sh [专辑子路径,默认全部]
#   例: deploy/upload-media.sh audio/xiyouji
set -euo pipefail
export LC_ALL=en_US.UTF-8 LANG=en_US.UTF-8

SERVER="root@159.195.18.74"
LIB_ROOT="/opt/1panel/apps/openresty/openresty/www/media/library"
SUB="${1:-}"

cd "$(dirname "$0")/../media-library"
ssh "$SERVER" "mkdir -p '$LIB_ROOT/$SUB'"
# -P: 显示进度 + 断点续传;不加 --delete,导入是只增操作
rsync -aP "./$SUB/" "$SERVER:$LIB_ROOT/$SUB/"

TOKEN="$(ssh "$SERVER" "grep '^ADMIN_TOKEN=' /opt/media-api/env | cut -d= -f2")"
ssh "$SERVER" "curl -sf -X POST -H 'X-Admin-Token: $TOKEN' http://127.0.0.1:8081/api/media/refresh"
echo
echo "✅ 上传完成并已刷新专辑列表"
