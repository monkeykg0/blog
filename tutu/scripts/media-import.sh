#!/usr/bin/env bash
# media-import.sh — 通用媒体专辑导入脚本
#
# 把一个装满音频文件的目录导入为标准专辑结构:
#   media-library/<type>/<album_id>/
#   ├── album.json      元数据(标题/集数/时长)
#   ├── cover.jpg       封面(可选,--cover 指定)
#   └── 001.mp3 …       硬链接(不占额外磁盘,原文件不动)
#
# 用法:
#   scripts/media-import.sh <源目录> <专辑ID> <专辑标题> [选项]
# 选项:
#   --artist  <名字>   演播者,默认空
#   --cover   <图片>   封面文件,复制为 cover.<ext>
#   --type    <类型>   audio|video,默认 audio
# 示例:
#   scripts/media-import.sh 西游记 xiyouji "凯叔讲西游记" --artist 凯叔
#
# 文件名规则:取开头数字作为集号排序;标题为去掉集号、扩展名、
# "+"、以及重复专辑名前缀后的剩余部分。
set -euo pipefail
# 强制 UTF-8:Claude Code/CI 等环境默认 C locale,bash 会按字节截取中文导致乱码
export LC_ALL=en_US.UTF-8 LANG=en_US.UTF-8

die() { echo "❌ $1" >&2; exit 1; }

[ $# -ge 3 ] || die "用法: $0 <源目录> <专辑ID> <专辑标题> [--artist X] [--cover 图片] [--type audio]"
SRC="$1"; ALBUM_ID="$2"; ALBUM_TITLE="$3"; shift 3

ARTIST=""; COVER=""; TYPE="audio"
while [ $# -gt 0 ]; do
  case "$1" in
    --artist) ARTIST="$2"; shift 2 ;;
    --cover)  COVER="$2";  shift 2 ;;
    --type)   TYPE="$2";   shift 2 ;;
    *) die "未知选项: $1" ;;
  esac
done

[ -d "$SRC" ] || die "源目录不存在: $SRC"
[[ "$ALBUM_ID" =~ ^[a-z0-9-]+$ ]] || die "专辑ID只能用小写字母/数字/连字符: $ALBUM_ID"
command -v ffprobe >/dev/null || die "需要 ffprobe (brew install ffmpeg)"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="$ROOT/media-library/$TYPE/$ALBUM_ID"
mkdir -p "$DEST"

# ---------- 收集源文件,按开头数字排序 ----------
TSV="$(mktemp)"
trap 'rm -f "$TSV"' EXIT
shopt -s nullglob
for f in "$SRC"/*.mp3 "$SRC"/*.m4a "$SRC"/*.m4b "$SRC"/*.aac "$SRC"/*.flac "$SRC"/*.wav; do
  base="$(basename "$f")"
  num="$(echo "$base" | grep -oE '^[0-9]+' || true)"
  # 无编号文件(如"序")排到最前
  [ -n "$num" ] || num=0
  printf '%s\t%s\n' "$num" "$f" >> "$TSV"
done
[ -s "$TSV" ] || die "源目录中没有找到带数字编号的音频文件"

TOTAL=$(wc -l < "$TSV" | tr -d ' ')
echo "▶ 导入《${ALBUM_TITLE}》(${ALBUM_ID}): ${TOTAL} 个文件"

# ---------- 逐个处理:硬链接 + 清洗标题 + 提取时长 ----------
META="$(mktemp)"
i=0
sort -n "$TSV" | while IFS=$'\t' read -r num src; do
  i=$((i + 1))
  n=$(printf '%03d' "$i")
  ext="${src##*.}"
  dest="$DEST/$n.$ext"
  # 硬链接(同卷零空间);跨卷退化为复制
  [ -e "$dest" ] && rm -f "$dest"
  ln "$src" "$dest" 2>/dev/null || cp "$src" "$dest"

  # 标题清洗:去扩展名 → 去开头数字 → 去"+" → 去重复的专辑名前缀 → 修剪
  base="$(basename "$src")"
  title="${base%.*}"
  title="$(echo "$title" | sed -E 's/^[0-9]+[. 　]*//; s/\+/ /g')"
  # 去重复的专辑名前缀(macOS bash 3.2 不支持 ${var#"$var"} 嵌套引号,用长度截取)
  case "$title" in
    "$ALBUM_TITLE"*) title="${title:${#ALBUM_TITLE}}" ;;
  esac
  # 常见情况:文件名内嵌书名(如"西游记计捉黑熊怪",而专辑叫"凯叔讲西游记")
  short="$(echo "$ALBUM_TITLE" | sed -E 's/^凯叔讲?|^.*讲//')"
  if [ -n "$short" ]; then
    case "$title" in
      "$short"*) title="${title:${#short}}" ;;
    esac
  fi
  title="$(echo "$title" | sed -E 's/[ 　]+（/（/g; s/[ 　]+/ /g; s/^[. 　]+|[. 　]+$//g')"
  [ -n "$title" ] || title="第 $i 集"

  dur="$(ffprobe -v quiet -show_entries format=duration -of csv=p=0 "$dest" | cut -d. -f1)"
  printf '%s\t%s.%s\t%s\t%s\n' "$i" "$n" "$ext" "$title" "${dur:-0}" >> "$META"
  printf '\r  处理中 %d/%d' "$i" "$TOTAL" >&2
done
echo >&2

# ---------- 封面 ----------
COVER_FILE=""
if [ -n "$COVER" ]; then
  [ -f "$COVER" ] || die "封面文件不存在: $COVER"
  cext="${COVER##*.}"
  cp "$COVER" "$DEST/cover.$cext"
  COVER_FILE="cover.$cext"
  echo "  封面: $COVER_FILE"
fi

# ---------- 生成 album.json ----------
ALBUM_ID="$ALBUM_ID" ALBUM_TITLE="$ALBUM_TITLE" ARTIST="$ARTIST" TYPE="$TYPE" \
COVER_FILE="$COVER_FILE" META_FILE="$META" DEST="$DEST" python3 - <<'PY'
import json, os, datetime

tracks = []
with open(os.environ["META_FILE"], encoding="utf-8") as f:
    for line in f:
        n, fname, title, dur = line.rstrip("\n").split("\t")
        tracks.append({"n": int(n), "file": fname, "title": title, "duration": int(dur)})

album = {
    "id": os.environ["ALBUM_ID"],
    "type": os.environ["TYPE"],
    "title": os.environ["ALBUM_TITLE"],
    "artist": os.environ["ARTIST"],
    "cover": os.environ["COVER_FILE"] or None,
    "trackCount": len(tracks),
    "totalDuration": sum(t["duration"] for t in tracks),
    # 完整时间戳:书架按此排序,同日导入两个专辑也能保持先来后到
    "updatedAt": datetime.datetime.now().isoformat(timespec="seconds"),
    "tracks": tracks,
}
path = os.path.join(os.environ["DEST"], "album.json")
with open(path, "w", encoding="utf-8") as f:
    json.dump(album, f, ensure_ascii=False, indent=2)
print(f"  album.json: {len(tracks)} 集,总时长 {album['totalDuration']//3600} 小时 {album['totalDuration']%3600//60} 分")
PY

rm -f "$META"
echo "✅ 完成 → $DEST"
echo "   下一步: deploy/deploy-media.sh 上传,然后 POST /api/media/refresh"
