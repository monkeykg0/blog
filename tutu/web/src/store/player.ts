import { create } from "zustand";
import { getAlbum, fileUrl } from "../lib/api";
import * as progress from "../lib/progress";
import type { Album, EpState, ProgressData, Track } from "../types";

// ============ 全局唯一音频元素(跨路由不中断) ============
const audio = new Audio();
audio.preload = "metadata";

export type SleepMode = null | { kind: "minutes"; until: number } | { kind: "eot" };

interface PlayerState {
  album: Album | null;
  track: Track | null;
  playing: boolean;
  position: number;
  duration: number;
  rate: number;
  sleep: SleepMode;
  /** 每集进度,key 为集号 */
  eps: Record<string, EpState>;
  loading: boolean;

  openAlbum: (albumId: string) => Promise<Album>;
  playTrack: (n: number, pos?: number) => void;
  /** 恢复上次进度播放;无进度则从第一集开始 */
  resume: () => void;
  toggle: () => void;
  next: () => void;
  prev: () => void;
  seekTo: (sec: number) => void;
  seekBy: (delta: number) => void;
  setRate: (r: number) => void;
  setSleep: (m: SleepMode) => void;
}

let sleepTimer: ReturnType<typeof setTimeout> | undefined;

export const usePlayer = create<PlayerState>()((set, get) => {
  // ---------- 持久化 ----------
  const snapshot = (): ProgressData | null => {
    const { album, track, position, rate, eps } = get();
    if (!album) return null;
    return {
      v: 1,
      cur: track ? { n: track.n, pos: Math.floor(position) } : null,
      eps,
      rate,
      savedAt: Date.now(),
    };
  };
  const persist = (immediate = false) => {
    const { album } = get();
    const data = snapshot();
    if (album && data) progress.push(album.id, data, immediate);
  };

  // ---------- 音频事件(仅绑定一次) ----------
  audio.addEventListener("timeupdate", () => {
    const { track, eps } = get();
    const pos = audio.currentTime;
    set({ position: pos });
    if (track) {
      eps[String(track.n)] = { ...eps[String(track.n)], pos: Math.floor(pos) };
      persist();
    }
  });
  audio.addEventListener("durationchange", () => set({ duration: audio.duration || 0 }));
  audio.addEventListener("play", () => set({ playing: true }));
  audio.addEventListener("pause", () => {
    set({ playing: false });
    persist(true);
  });
  audio.addEventListener("ended", () => {
    const { track, eps, sleep, next } = get();
    if (track) {
      set({ eps: { ...eps, [String(track.n)]: { pos: 0, done: true } } });
    }
    persist(true);
    if (sleep?.kind === "eot") {
      get().setSleep(null);
      return; // 播完本集睡觉 🌙
    }
    next();
  });
  audio.addEventListener("error", () => set({ playing: false, loading: false }));

  // 切后台/关页面立即保存
  if (typeof document !== "undefined") {
    document.addEventListener("visibilitychange", () => {
      if (document.visibilityState === "hidden") {
        persist(true);
        progress.flush();
      }
    });
  }

  // ---------- Media Session(锁屏控制) ----------
  const updateMediaSession = () => {
    if (!("mediaSession" in navigator)) return;
    const { album, track } = get();
    if (!album || !track) return;
    navigator.mediaSession.metadata = new MediaMetadata({
      title: track.title,
      artist: album.artist || album.title,
      album: album.title,
      artwork: album.cover
        ? [{ src: fileUrl(album, album.cover), sizes: "512x512" }]
        : [{ src: new URL("icons/icon-512.png", document.baseURI).toString(), sizes: "512x512", type: "image/png" }],
    });
    navigator.mediaSession.setActionHandler("play", () => get().toggle());
    navigator.mediaSession.setActionHandler("pause", () => get().toggle());
    navigator.mediaSession.setActionHandler("previoustrack", () => get().prev());
    navigator.mediaSession.setActionHandler("nexttrack", () => get().next());
    navigator.mediaSession.setActionHandler("seekbackward", () => get().seekBy(-15));
    navigator.mediaSession.setActionHandler("seekforward", () => get().seekBy(15));
    navigator.mediaSession.setActionHandler("seekto", (e) => {
      if (e.seekTime != null) get().seekTo(e.seekTime);
    });
  };

  return {
    album: null,
    track: null,
    playing: false,
    position: 0,
    duration: 0,
    rate: 1,
    sleep: null,
    eps: {},
    loading: false,

    async openAlbum(albumId) {
      const cur = get().album;
      if (cur?.id === albumId) return cur; // 已打开(可能正在播),不打断
      set({ loading: true });
      try {
        const [album, saved] = await Promise.all([
          getAlbum(albumId),
          progress.loadMerged(albumId),
        ]);
        set({
          album,
          eps: saved?.eps ?? {},
          rate: saved?.rate ?? 1,
          track: null,
          position: saved?.cur?.pos ?? 0,
          duration: 0,
          loading: false,
        });
        // 恢复"当前集"信息供 UI 展示(不自动播,尊重移动端策略)
        if (saved?.cur) {
          const t = album.tracks.find((t) => t.n === saved.cur!.n);
          if (t) set({ track: t, duration: t.duration });
        }
        return album;
      } catch (e) {
        set({ loading: false });
        throw e;
      }
    },

    playTrack(n, pos) {
      const { album, eps, rate } = get();
      if (!album) return;
      const t = album.tracks.find((t) => t.n === n);
      if (!t) return;
      const startPos = pos ?? eps[String(n)]?.pos ?? 0;
      // 快听完的(剩<10s)从头播
      const effective = t.duration - startPos < 10 ? 0 : startPos;
      audio.src = fileUrl(album, t.file);
      audio.currentTime = effective;
      audio.playbackRate = rate;
      set({ track: t, position: effective, duration: t.duration });
      void audio.play();
      updateMediaSession();
    },

    resume() {
      const { album, track, position } = get();
      if (!album) return;
      if (track) {
        // openAlbum 已定位到上次的集
        get().playTrack(track.n, position);
      } else {
        get().playTrack(album.tracks[0]?.n ?? 1);
      }
    },

    toggle() {
      const { track } = get();
      if (!track) {
        get().resume();
        return;
      }
      if (audio.paused) void audio.play();
      else audio.pause();
    },

    next() {
      const { album, track } = get();
      if (!album || !track) return;
      const i = album.tracks.findIndex((t) => t.n === track.n);
      const nt = album.tracks[i + 1];
      if (nt) get().playTrack(nt.n);
    },

    prev() {
      const { album, track } = get();
      if (!album || !track) return;
      // 播了 3 秒以上,"上一集"先回本集开头(播放器惯例)
      if (audio.currentTime > 3) {
        get().seekTo(0);
        return;
      }
      const i = album.tracks.findIndex((t) => t.n === track.n);
      const pt = album.tracks[i - 1];
      if (pt) get().playTrack(pt.n);
    },

    seekTo(sec) {
      audio.currentTime = Math.max(0, Math.min(sec, audio.duration || sec));
      set({ position: audio.currentTime });
    },

    seekBy(delta) {
      get().seekTo(audio.currentTime + delta);
    },

    setRate(r) {
      audio.playbackRate = r;
      set({ rate: r });
      persist();
    },

    setSleep(m) {
      if (sleepTimer) clearTimeout(sleepTimer);
      sleepTimer = undefined;
      set({ sleep: m });
      if (m?.kind === "minutes") {
        sleepTimer = setTimeout(() => {
          audio.pause();
          set({ sleep: null });
        }, m.until - Date.now());
      }
    },
  };
});
