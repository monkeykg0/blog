import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import CoverArt from "../components/CoverArt";
import Rabbit from "../components/Rabbit";
import { getLibrary } from "../lib/api";
import { fmtDurationCN } from "../lib/format";
import { loadLocal } from "../lib/progress";
import type { AlbumSummary } from "../types";

const TABS = [
  { key: "audio", label: "听故事", ready: true },
  { key: "video", label: "看动画", ready: false },
  { key: "image", label: "翻相册", ready: false },
] as const;

/** 书架页 */
export default function Shelf() {
  const [albums, setAlbums] = useState<AlbumSummary[] | null>(null);
  const [error, setError] = useState(false);
  const [tab, setTab] = useState<(typeof TABS)[number]["key"]>("audio");

  useEffect(() => {
    getLibrary()
      .then((r) => setAlbums(r.albums))
      .catch(() => setError(true));
  }, []);

  const shown = albums?.filter((a) => a.type === tab) ?? [];

  return (
    <div className="mx-auto max-w-5xl px-4 pb-32 pt-6 sm:px-6">
      {/* 头部 */}
      <header className="mb-6 flex items-center gap-3">
        <div className="h-14 w-14 animate-bounce-soft">
          <Rabbit mood="idle" className="h-full w-full" />
        </div>
        <div>
          <h1 className="text-2xl font-bold tracking-wide">兔兔听书屋</h1>
          <p className="text-sm text-ink-soft">今天想听什么故事呀?</p>
        </div>
      </header>

      {/* 分类 Tab */}
      <nav className="mb-6 flex gap-2">
        {TABS.map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={`rounded-full px-5 py-2.5 text-sm font-bold ring-2 transition-all active:scale-95 ${
              tab === t.key
                ? "bg-carrot text-white ring-carrot shadow-sticker"
                : "bg-paper text-ink-soft ring-blush"
            }`}
          >
            {t.label}
          </button>
        ))}
      </nav>

      {/* 内容 */}
      {error && (
        <Empty text="书架连不上啦,检查一下网络再试试 🐰" />
      )}
      {!error && albums === null && <Empty text="兔兔正在搬书上架…" />}
      {!error && albums !== null && shown.length === 0 && (
        <Empty
          text={
            TABS.find((t) => t.key === tab)?.ready
              ? "这里还空空的,等着装故事哦"
              : "这个房间还在装修中,先去听故事吧 🐰"
          }
        />
      )}

      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 sm:gap-5 lg:grid-cols-4">
        {shown.map((a) => (
          <AlbumCard key={a.id} album={a} />
        ))}
      </div>
    </div>
  );
}

function AlbumCard({ album }: { album: AlbumSummary }) {
  const saved = loadLocal(album.id);
  const startedCount = saved ? Object.keys(saved.eps).length : 0;
  return (
    <Link
      to={`/album/${album.id}`}
      className="group block rounded-3xl bg-paper p-3 shadow-sticker ring-2 ring-blush transition-transform hover:-translate-y-1 active:scale-95"
    >
      <CoverArt album={album} className="aspect-square w-full" />
      <div className="mt-3 space-y-0.5 px-1">
        <div className="truncate font-bold">{album.title}</div>
        <div className="text-xs text-ink-soft">
          {album.trackCount} 集 · {fmtDurationCN(album.totalDuration)}
        </div>
        {startedCount > 0 && (
          <div className="pt-1 text-xs font-bold text-leaf-deep">▶ 继续听</div>
        )}
      </div>
    </Link>
  );
}

function Empty({ text }: { text: string }) {
  return (
    <div className="flex flex-col items-center gap-4 py-16 text-ink-soft">
      <div className="h-24 w-24 opacity-70">
        <Rabbit mood="paused" className="h-full w-full" />
      </div>
      <p className="text-sm">{text}</p>
    </div>
  );
}
