import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import CoverArt from "../components/CoverArt";
import PlayerControls from "../components/PlayerControls";
import Rabbit from "../components/Rabbit";
import TrackList from "../components/TrackList";
import { fmtDurationCN, fmtTime } from "../lib/format";
import { usePlayer } from "../store/player";

/** 专辑播放页 */
export default function AlbumPage() {
  const { id } = useParams<{ id: string }>();
  const album = usePlayer((s) => s.album);
  const track = usePlayer((s) => s.track);
  const playing = usePlayer((s) => s.playing);
  const position = usePlayer((s) => s.position);
  const openAlbum = usePlayer((s) => s.openAlbum);
  const resume = usePlayer((s) => s.resume);
  const [error, setError] = useState(false);

  useEffect(() => {
    if (id) openAlbum(id).catch(() => setError(true));
  }, [id, openAlbum]);

  if (error) {
    return (
      <Centered>
        <Rabbit mood="paused" className="h-24 w-24" />
        <p className="text-ink-soft">这本书找不到啦,回书架看看吧</p>
        <BackLink />
      </Centered>
    );
  }
  if (!album || album.id !== id) {
    return (
      <Centered>
        <Rabbit mood="idle" className="h-24 w-24 animate-bounce-soft" />
        <p className="text-ink-soft">兔兔正在翻书…</p>
      </Centered>
    );
  }

  const started = track !== null;

  return (
    <div className="mx-auto max-w-5xl px-4 pb-10 pt-4 sm:px-6">
      <div className="mb-4">
        <BackLink />
      </div>

      <div className="grid gap-5 lg:grid-cols-[minmax(0,5fr)_minmax(0,6fr)] lg:items-start">
        {/* 左栏:封面 + 播放器 */}
        <div className="rounded-3xl bg-paper p-5 shadow-sticker ring-2 ring-blush sm:p-6">
          <div className="mb-5 flex items-center gap-4">
            <CoverArt album={album} className="h-24 w-24 shrink-0 sm:h-28 sm:w-28" />
            <div className="min-w-0">
              <h1 className="text-xl font-bold leading-snug sm:text-2xl">{album.title}</h1>
              <p className="mt-1 text-sm text-ink-soft">
                {album.artist && `${album.artist} · `}
                {album.trackCount} 集 · {fmtDurationCN(album.totalDuration)}
              </p>
            </div>
            {/* 陪听兔兔 */}
            <div className="ml-auto hidden h-16 w-16 shrink-0 sm:block">
              <Rabbit mood={playing ? "playing" : started ? "paused" : "idle"} className="h-full w-full" />
            </div>
          </div>

          {/* 当前集 / 继续听 */}
          {started ? (
            <div className="mb-4 truncate rounded-2xl bg-blush-soft px-4 py-2.5 text-sm font-bold">
              正在听:{track.n}. {track.title}
            </div>
          ) : (
            <button
              onClick={resume}
              className="mb-4 w-full rounded-2xl bg-gradient-to-b from-leaf to-leaf-deep py-3.5 text-base font-bold text-white shadow-sticker transition-transform active:scale-95"
            >
              ▶ 开始听第一集
            </button>
          )}
          {/* 上次进度提示(有记录且未开播时) */}
          <ResumeHint />

          <PlayerControls />
        </div>

        {/* 右栏:章节列表 */}
        <TrackList album={album} />
      </div>
    </div>
  );

  function ResumeHint() {
    if (!track || playing || position <= 5) return null;
    return (
      <button
        onClick={resume}
        className="mb-4 w-full rounded-2xl bg-gradient-to-b from-carrot to-carrot-deep py-3.5 text-base font-bold text-white shadow-sticker transition-transform active:scale-95"
      >
        ▶ 继续听 · {track.title} {fmtTime(position)}
      </button>
    );
  }
}

function BackLink() {
  return (
    <Link
      to="/"
      className="inline-flex items-center gap-1.5 rounded-full bg-paper px-4 py-2 text-sm font-bold text-ink-soft ring-2 ring-blush transition-transform active:scale-95"
    >
      ← 回书架
    </Link>
  );
}

function Centered({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-[70dvh] flex-col items-center justify-center gap-4">{children}</div>
  );
}
