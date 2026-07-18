import { Link } from "react-router-dom";
import { fmtTime } from "../lib/format";
import { usePlayer } from "../store/player";
import Rabbit from "./Rabbit";

/** 书架页底部的迷你播放条:切页面音频不断 */
export default function MiniPlayer() {
  const album = usePlayer((s) => s.album);
  const track = usePlayer((s) => s.track);
  const playing = usePlayer((s) => s.playing);
  const position = usePlayer((s) => s.position);
  const duration = usePlayer((s) => s.duration);
  const toggle = usePlayer((s) => s.toggle);

  if (!album || !track) return null;
  const pct = duration > 0 ? (position / duration) * 100 : 0;

  return (
    <div className="fixed inset-x-0 bottom-0 z-20 px-3 pb-[max(0.75rem,env(safe-area-inset-bottom))]">
      <div className="mx-auto flex max-w-xl items-center gap-3 rounded-3xl bg-paper/95 p-2.5 pr-4 shadow-sticker-lg ring-2 ring-blush backdrop-blur">
        <Link to={`/album/${album.id}`} className="flex min-w-0 flex-1 items-center gap-3">
          <div className="h-11 w-11 shrink-0">
            <Rabbit mood={playing ? "playing" : "paused"} className="h-full w-full" />
          </div>
          <div className="min-w-0">
            <div className="truncate text-sm font-bold">
              {track.n}. {track.title}
            </div>
            <div className="truncate text-xs text-ink-soft">
              {album.title} · {fmtTime(position)} / {fmtTime(duration)}
            </div>
            {/* 细进度线 */}
            <div className="mt-1 h-1 w-full overflow-hidden rounded-full bg-blush-soft">
              <div className="h-full rounded-full bg-carrot" style={{ width: `${pct}%` }} />
            </div>
          </div>
        </Link>
        <button
          onClick={toggle}
          aria-label={playing ? "暂停" : "播放"}
          className="flex h-12 w-12 shrink-0 items-center justify-center rounded-full bg-carrot text-white shadow-sticker transition-transform active:scale-90"
        >
          {playing ? <PauseIcon /> : <PlayIcon />}
        </button>
      </div>
    </div>
  );
}

export function PlayIcon({ className = "h-5 w-5" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" className={className} aria-hidden>
      <path d="M8.5 5.6v12.8c0 .9 1 1.5 1.8 1L20 13c.8-.5.8-1.6 0-2.1L10.3 4.6c-.8-.5-1.8.1-1.8 1z" />
    </svg>
  );
}

export function PauseIcon({ className = "h-5 w-5" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" className={className} aria-hidden>
      <rect x="6" y="5" width="4.5" height="14" rx="1.8" />
      <rect x="13.5" y="5" width="4.5" height="14" rx="1.8" />
    </svg>
  );
}
