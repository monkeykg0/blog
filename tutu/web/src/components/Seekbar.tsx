import { useRef, useState } from "react";

interface SeekbarProps {
  position: number;
  duration: number;
  onSeek: (sec: number) => void;
}

/** 胡萝卜进度条:拖动滑块是一枚小胡萝卜 🥕 */
export default function Seekbar({ position, duration, onSeek }: SeekbarProps) {
  const barRef = useRef<HTMLDivElement>(null);
  const [dragPos, setDragPos] = useState<number | null>(null);

  const shown = dragPos ?? position;
  const pct = duration > 0 ? Math.min(100, (shown / duration) * 100) : 0;

  const posFromEvent = (e: React.PointerEvent) => {
    const rect = barRef.current!.getBoundingClientRect();
    const ratio = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
    return ratio * duration;
  };

  return (
    <div
      ref={barRef}
      className="group relative h-9 cursor-pointer touch-none select-none"
      role="slider"
      aria-valuemin={0}
      aria-valuemax={duration}
      aria-valuenow={Math.floor(shown)}
      aria-label="播放进度"
      onPointerDown={(e) => {
        e.currentTarget.setPointerCapture(e.pointerId);
        setDragPos(posFromEvent(e));
      }}
      onPointerMove={(e) => {
        if (dragPos !== null) setDragPos(posFromEvent(e));
      }}
      onPointerUp={(e) => {
        if (dragPos !== null) onSeek(posFromEvent(e));
        setDragPos(null);
      }}
    >
      {/* 轨道 */}
      <div className="absolute top-1/2 h-3.5 w-full -translate-y-1/2 rounded-full bg-blush-soft ring-2 ring-blush" />
      {/* 已播(胡萝卜色) */}
      <div
        className="absolute top-1/2 h-3.5 -translate-y-1/2 rounded-full bg-gradient-to-r from-carrot to-carrot-deep"
        style={{ width: `${pct}%` }}
      />
      {/* 胡萝卜滑块 */}
      <div
        className="absolute top-1/2 -translate-x-1/2 -translate-y-1/2 transition-transform group-active:scale-125"
        style={{ left: `${pct}%` }}
      >
        <svg viewBox="0 0 28 28" className="h-7 w-7 drop-shadow-sm" aria-hidden>
          <g transform="rotate(45 14 14)">
            <path d="M14 4 L19 16 Q14 22 9 16 Z" fill="#ff8a3d" stroke="#f06f1f" strokeWidth="1.5" />
            <path d="M14 4 q-4 -3 -6 -1 q2 1 3 2 q-3 0 -4 2 q3 0 5 1 Z" fill="#7bc47f" />
          </g>
        </svg>
      </div>
    </div>
  );
}
