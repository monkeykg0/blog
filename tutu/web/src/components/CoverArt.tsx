import { fileUrl } from "../lib/api";
import type { AlbumSummary } from "../types";
import Rabbit from "./Rabbit";

interface CoverArtProps {
  album: AlbumSummary;
  className?: string;
}

/** 专辑封面:有 cover 用图,没有就生成一张可爱的兔兔封面 */
export default function CoverArt({ album, className = "" }: CoverArtProps) {
  if (album.cover) {
    return (
      <img
        src={fileUrl(album, album.cover)}
        alt={album.title}
        className={`rounded-3xl object-cover ring-4 ring-white shadow-sticker-lg ${className}`}
        draggable={false}
      />
    );
  }
  // 兜底封面:主标题取书名末两字大字排版
  const big = album.title.replace(/^凯叔讲?/, "").slice(0, 4);
  return (
    <div
      className={`relative flex flex-col items-center justify-center gap-1 overflow-hidden rounded-3xl bg-gradient-to-br from-sky via-blush-soft to-blush ring-4 ring-white shadow-sticker-lg ${className}`}
    >
      <Rabbit mood="idle" className="h-1/3 w-1/3" />
      <div className="px-2 text-center font-bold leading-tight text-ink" style={{ fontSize: "min(1.4rem, 5.5cqw, 22px)" }}>
        {big}
      </div>
      {album.artist && <div className="text-[10px] text-ink-soft">{album.artist}</div>}
    </div>
  );
}
