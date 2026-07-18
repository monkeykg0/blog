interface RabbitProps {
  /** playing: 竖耳摇摆;paused: 打瞌睡 zzz;idle: 安静待着 */
  mood: "playing" | "paused" | "idle";
  className?: string;
}

/** 兔兔吉祥物:纯 SVG,随播放状态变化 */
export default function Rabbit({ mood, className = "" }: RabbitProps) {
  const playing = mood === "playing";
  return (
    <div className={`relative ${playing ? "animate-hop" : ""} ${className}`}>
      <svg viewBox="0 0 120 120" className="h-full w-full" aria-hidden>
        {/* 左耳 */}
        <g
          className={playing ? "animate-ear-l" : ""}
          style={{ transformOrigin: "42px 52px" }}
        >
          <ellipse cx="40" cy="26" rx="11" ry="26" fill="#fff" stroke="#4a3728" strokeWidth="3" />
          <ellipse cx="40" cy="28" rx="5.5" ry="18" fill="#ffd9c9" />
        </g>
        {/* 右耳 */}
        <g
          className={playing ? "animate-ear-r" : ""}
          style={{ transformOrigin: "78px 52px" }}
        >
          <ellipse cx="80" cy="26" rx="11" ry="26" fill="#fff" stroke="#4a3728" strokeWidth="3" />
          <ellipse cx="80" cy="28" rx="5.5" ry="18" fill="#ffd9c9" />
        </g>
        {/* 脸 */}
        <circle cx="60" cy="72" r="38" fill="#fff" stroke="#4a3728" strokeWidth="3" />
        {/* 腮红 */}
        <ellipse cx="38" cy="80" rx="7" ry="4.5" fill="#ffc4ab" />
        <ellipse cx="82" cy="80" rx="7" ry="4.5" fill="#ffc4ab" />
        {/* 眼睛:睡着时是弯线 */}
        {mood === "paused" ? (
          <>
            <path d="M40 68 q6 6 12 0" stroke="#4a3728" strokeWidth="3" fill="none" strokeLinecap="round" />
            <path d="M68 68 q6 6 12 0" stroke="#4a3728" strokeWidth="3" fill="none" strokeLinecap="round" />
          </>
        ) : (
          <>
            <circle cx="46" cy="68" r="4.5" fill="#4a3728" />
            <circle cx="74" cy="68" r="4.5" fill="#4a3728" />
            <circle cx="47.5" cy="66.5" r="1.6" fill="#fff" />
            <circle cx="75.5" cy="66.5" r="1.6" fill="#fff" />
          </>
        )}
        {/* 鼻子和嘴 */}
        <path d="M56 79 q4 -4 8 0 q-4 5 -8 0" fill="#ff8a3d" />
        <path d="M60 82 v4 M60 86 q-5 6 -10 2 M60 86 q5 6 10 2" stroke="#4a3728" strokeWidth="2.5" fill="none" strokeLinecap="round" />
      </svg>
      {mood === "paused" && (
        <span className="animate-zzz absolute -right-1 top-0 text-sm font-bold text-ink-soft select-none">
          z Z
        </span>
      )}
    </div>
  );
}
