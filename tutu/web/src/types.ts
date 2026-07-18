export interface Track {
  n: number;
  file: string;
  title: string;
  duration: number; // 秒
}

export interface AlbumSummary {
  id: string;
  type: "audio" | "video" | "image";
  title: string;
  artist: string;
  cover: string | null;
  trackCount: number;
  totalDuration: number; // 秒
  updatedAt: string;
}

export interface Album extends AlbumSummary {
  tracks: Track[];
}

/** 单集收听状态 */
export interface EpState {
  pos: number; // 上次位置(秒)
  done?: boolean; // 已听完
}

/** 一个专辑的完整播放进度(本地与云端同构,savedAt 新者胜) */
export interface ProgressData {
  v: 1;
  cur: { n: number; pos: number } | null;
  eps: Record<string, EpState>;
  rate: number;
  savedAt: number; // Date.now()
}
