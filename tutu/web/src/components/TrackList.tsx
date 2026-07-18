import { useMemo, useState } from "react";
import { fmtTime } from "../lib/format";
import { usePlayer } from "../store/player";
import type { Album } from "../types";

/** 章节列表:搜索、当前集高亮、已听完盖萝卜章、半途显示小进度 */
export default function TrackList({ album }: { album: Album }) {
  const [q, setQ] = useState("");
  const trackN = usePlayer((s) => s.track?.n);
  const playing = usePlayer((s) => s.playing);
  const eps = usePlayer((s) => s.eps);
  const playTrack = usePlayer((s) => s.playTrack);

  const shown = useMemo(
    () => (q ? album.tracks.filter((t) => t.title.includes(q) || String(t.n) === q) : album.tracks),
    [album.tracks, q],
  );

  return (
    <div className="rounded-3xl bg-paper p-3 shadow-sticker ring-2 ring-blush sm:p-4">
      <div className="mb-3 flex items-center gap-3 px-1">
        <h2 className="shrink-0 text-lg font-bold">故事列表</h2>
        <input
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="找一集…"
          className="w-full min-w-0 rounded-full bg-blush-soft px-4 py-2 text-sm outline-none ring-2 ring-transparent placeholder:text-ink-soft/60 focus:ring-carrot"
        />
      </div>

      <ol className="max-h-[52dvh] space-y-1 overflow-y-auto overscroll-contain pr-1 lg:max-h-[60dvh]">
        {shown.map((t) => {
          const isCur = t.n === trackN;
          const ep = eps[String(t.n)];
          const pct = ep && !ep.done && t.duration > 0 ? Math.min(99, (ep.pos / t.duration) * 100) : 0;
          return (
            <li key={t.n}>
              <button
                onClick={() => playTrack(t.n)}
                className={`flex w-full items-center gap-3 rounded-2xl px-3 py-2.5 text-left transition-colors ${
                  isCur ? "bg-blush ring-2 ring-carrot" : "hover:bg-blush-soft active:bg-blush"
                }`}
              >
                {/* 集号徽章 */}
                <span
                  className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-sm font-bold ${
                    isCur ? "bg-carrot text-white" : "bg-blush-soft text-ink-soft"
                  }`}
                >
                  {isCur && playing ? (
                    <EqIcon />
                  ) : (
                    t.n
                  )}
                </span>

                <span className="min-w-0 flex-1">
                  <span className={`block truncate ${isCur ? "font-bold" : ""}`}>{t.title}</span>
                  {pct > 0 && (
                    <span className="mt-1 block h-1.5 w-full max-w-40 overflow-hidden rounded-full bg-white">
                      <span className="block h-full rounded-full bg-leaf" style={{ width: `${pct}%` }} />
                    </span>
                  )}
                </span>

                <span className="shrink-0 text-xs text-ink-soft">{fmtTime(t.duration)}</span>
                {ep?.done && <DoneStamp />}
              </button>
            </li>
          );
        })}
        {shown.length === 0 && (
          <li className="py-8 text-center text-sm text-ink-soft">没有找到这一集哦 🐰</li>
        )}
      </ol>
    </div>
  );
}

/** 播放中的小跳动条 */
function EqIcon() {
  return (
    <span className="flex h-3.5 items-end gap-[2px]" aria-label="播放中">
      {[0, 0.25, 0.5].map((d) => (
        <span
          key={d}
          className="w-[3px] rounded-full bg-white"
          style={{ animation: `hop 0.8s ${d}s ease-in-out infinite`, height: "100%" }}
        />
      ))}
    </span>
  );
}

/** 听完的萝卜印章 */
function DoneStamp() {
  return (
    <span className="shrink-0 -rotate-12 rounded-lg border-2 border-leaf px-1.5 py-0.5 text-[10px] font-bold text-leaf-deep">
      听完啦
    </span>
  );
}
