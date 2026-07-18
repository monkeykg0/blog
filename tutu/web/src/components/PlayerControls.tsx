import { useEffect, useState } from "react";
import { fmtTime } from "../lib/format";
import { usePlayer } from "../store/player";
import { PauseIcon, PlayIcon } from "./MiniPlayer";
import Seekbar from "./Seekbar";

const RATES = [1, 1.25, 1.5, 2, 0.75];

/** 播放主控:进度条 + 上一集/-15s/播放/+15s/下一集 + 倍速 + 睡觉倒计时 */
export default function PlayerControls() {
  const playing = usePlayer((s) => s.playing);
  const position = usePlayer((s) => s.position);
  const duration = usePlayer((s) => s.duration);
  const rate = usePlayer((s) => s.rate);
  const { toggle, next, prev, seekTo, seekBy, setRate } = usePlayer.getState();

  return (
    <div className="space-y-3">
      <Seekbar position={position} duration={duration} onSeek={seekTo} />
      <div className="-mt-1 flex justify-between text-xs font-bold text-ink-soft">
        <span>{fmtTime(position)}</span>
        <span>{fmtTime(duration)}</span>
      </div>

      <div className="flex flex-wrap items-center justify-center gap-3 sm:gap-4">
        {/* 主控一组:手机上独占一行 */}
        <div className="flex items-center gap-2 sm:gap-4">
          <IconBtn label="上一集" onClick={prev}>
            <svg viewBox="0 0 24 24" fill="currentColor" className="h-6 w-6" aria-hidden>
              <path d="M7 6a1 1 0 0 1 2 0v4.3l8.2-5.2c.8-.5 1.8.1 1.8 1v11.8c0 .9-1 1.5-1.8 1L9 13.7V18a1 1 0 1 1-2 0z" />
            </svg>
          </IconBtn>

          <IconBtn label="后退 15 秒" onClick={() => seekBy(-15)} small>
            <Skip15 back />
          </IconBtn>

          {/* 大播放键 */}
          <button
            onClick={toggle}
            aria-label={playing ? "暂停" : "播放"}
            className="flex h-16 w-16 items-center justify-center rounded-full bg-gradient-to-b from-carrot to-carrot-deep text-white shadow-sticker-lg ring-4 ring-white transition-transform active:scale-90 sm:h-18 sm:w-18"
          >
            {playing ? <PauseIcon className="h-8 w-8" /> : <PlayIcon className="ml-1 h-8 w-8" />}
          </button>

          <IconBtn label="前进 15 秒" onClick={() => seekBy(15)} small>
            <Skip15 />
          </IconBtn>

          <IconBtn label="下一集" onClick={next}>
            <svg viewBox="0 0 24 24" fill="currentColor" className="h-6 w-6" aria-hidden>
              <path d="M17 6a1 1 0 0 0-2 0v4.3L6.8 5.1C6 4.6 5 5.2 5 6.1v11.8c0 .9 1 1.5 1.8 1l8.2-5.2V18a1 1 0 1 0 2 0z" />
            </svg>
          </IconBtn>
        </div>

        {/* 换行:主控与辅助控件分两行,移动端桌面端一致 */}
        <div className="basis-full" />

        {/* 倍速:点按循环 */}
        <button
          onClick={() => setRate(RATES[(RATES.indexOf(rate) + 1) % RATES.length] ?? 1)}
          className="w-14 rounded-full bg-blush-soft py-2 text-sm font-bold text-ink-soft ring-2 ring-blush transition-transform active:scale-90"
          aria-label="播放速度"
        >
          {rate}×
        </button>

        <SleepButton />
      </div>
    </div>
  );
}

function IconBtn({
  label,
  onClick,
  small,
  children,
}: {
  label: string;
  onClick: () => void;
  small?: boolean;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      aria-label={label}
      className={`flex items-center justify-center rounded-full text-ink transition-transform active:scale-90 ${
        small ? "h-11 w-11 bg-blush-soft ring-2 ring-blush" : "h-12 w-12 hover:bg-blush-soft"
      }`}
    >
      {children}
    </button>
  );
}

function Skip15({ back = false }: { back?: boolean }) {
  return (
    <span className="relative flex items-center justify-center" aria-hidden>
      <svg
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        className={`h-7 w-7 ${back ? "-scale-x-100" : ""}`}
      >
        <path d="M12 4a8 8 0 1 1-7.4 5" strokeLinecap="round" />
        <path d="M4 3v6h6" strokeLinecap="round" strokeLinejoin="round" fill="none" />
      </svg>
      <span className="absolute text-[8.5px] font-bold">15</span>
    </span>
  );
}

/** 兔兔睡觉倒计时 🌙 */
function SleepButton() {
  const sleep = usePlayer((s) => s.sleep);
  const setSleep = usePlayer((s) => s.setSleep);
  const [open, setOpen] = useState(false);
  const [, tick] = useState(0);

  // 有倒计时时每秒刷新剩余分钟
  useEffect(() => {
    if (sleep?.kind !== "minutes") return;
    const id = setInterval(() => tick((n) => n + 1), 1000);
    return () => clearInterval(id);
  }, [sleep]);

  const remainMin =
    sleep?.kind === "minutes" ? Math.max(0, Math.ceil((sleep.until - Date.now()) / 60000)) : null;

  return (
    <div className="relative">
      <button
        onClick={() => setOpen((v) => !v)}
        aria-label="睡觉倒计时"
        className={`flex h-11 w-11 items-center justify-center rounded-full text-lg ring-2 transition-transform active:scale-90 ${
          sleep ? "bg-sky ring-sky" : "bg-blush-soft ring-blush"
        }`}
      >
        {sleep ? (remainMin !== null ? <span className="text-xs font-bold">{remainMin}分</span> : "🌙") : "🌙"}
      </button>

      {open && (
        <>
          <div className="fixed inset-0 z-30" onClick={() => setOpen(false)} />
          <div className="absolute bottom-14 right-0 z-40 w-44 rounded-2xl bg-paper p-2 shadow-sticker-lg ring-2 ring-blush">
            <div className="px-2 py-1 text-xs font-bold text-ink-soft">兔兔睡觉倒计时</div>
            {[15, 30, 45].map((m) => (
              <SleepOption
                key={m}
                label={`${m} 分钟后`}
                active={false}
                onClick={() => {
                  setSleep({ kind: "minutes", until: Date.now() + m * 60_000 });
                  setOpen(false);
                }}
              />
            ))}
            <SleepOption
              label="听完这集"
              active={sleep?.kind === "eot"}
              onClick={() => {
                setSleep({ kind: "eot" });
                setOpen(false);
              }}
            />
            {sleep && (
              <SleepOption
                label="不睡啦,取消"
                active={false}
                onClick={() => {
                  setSleep(null);
                  setOpen(false);
                }}
              />
            )}
          </div>
        </>
      )}
    </div>
  );
}

function SleepOption({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className={`block w-full rounded-xl px-3 py-2 text-left text-sm transition-colors hover:bg-blush-soft ${
        active ? "font-bold text-carrot-deep" : ""
      }`}
    >
      {label}
    </button>
  );
}
