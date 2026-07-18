import { getRemoteProgress, putRemoteProgress } from "./api";
import type { ProgressData } from "../types";

// 断点续播持久化:localStorage 即时 + 云端节流,savedAt 新者胜。
// 云端不可用时静默降级为纯本地(离线可用)。

const key = (albumId: string) => `tutu:progress:${albumId}`;

export function loadLocal(albumId: string): ProgressData | null {
  try {
    const raw = localStorage.getItem(key(albumId));
    if (!raw) return null;
    const data = JSON.parse(raw) as ProgressData;
    return data.v === 1 ? data : null;
  } catch {
    return null;
  }
}

export function saveLocal(albumId: string, data: ProgressData): void {
  try {
    localStorage.setItem(key(albumId), JSON.stringify(data));
  } catch {
    /* 存储满/隐私模式,忽略 */
  }
}

/** 打开专辑时调用:取本地与云端较新者 */
export async function loadMerged(albumId: string): Promise<ProgressData | null> {
  const local = loadLocal(albumId);
  try {
    const remote = await getRemoteProgress(albumId);
    if (remote.data && (!local || remote.data.savedAt > local.savedAt)) {
      saveLocal(albumId, remote.data); // 云端更新,回写本地
      return remote.data;
    }
  } catch {
    /* 离线/服务不可用 → 用本地 */
  }
  return local;
}

// ---- 云端推送节流:播放中每 10 秒最多一次,暂停/离开时立即 ----
let lastPush = 0;
let pending: { albumId: string; data: ProgressData } | null = null;

export function push(albumId: string, data: ProgressData, immediate = false): void {
  saveLocal(albumId, data);
  pending = { albumId, data };
  const now = Date.now();
  if (!immediate && now - lastPush < 10_000) return;
  flush();
}

export function flush(): void {
  if (!pending) return;
  lastPush = Date.now();
  const { albumId, data } = pending;
  pending = null;
  putRemoteProgress(albumId, data).catch(() => {
    /* 失败不重试,下个节拍会带最新数据再来 */
  });
}
