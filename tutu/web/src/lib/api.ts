import type { Album, AlbumSummary, ProgressData } from "../types";

// token 构建期注入(部署脚本传 VITE_MEDIA_TOKEN);页面本身已在隐秘路径之后
const TOKEN = import.meta.env.VITE_MEDIA_TOKEN ?? "dev-token";
const API = "/api/media";

async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API}${path}`, {
    ...init,
    headers: { "X-Media-Token": TOKEN, ...init?.headers },
  });
  if (!res.ok) throw new Error(`API ${path}: ${res.status}`);
  return res.json() as Promise<T>;
}

export function getLibrary(): Promise<{ albums: AlbumSummary[] }> {
  return fetchJSON("/library");
}

export function getAlbum(id: string): Promise<Album> {
  return fetchJSON(`/album/${id}`);
}

export function getRemoteProgress(
  albumId: string,
): Promise<{ data: ProgressData | null; updatedAt: string | null }> {
  return fetchJSON(`/progress/${albumId}`);
}

export function putRemoteProgress(albumId: string, data: ProgressData): Promise<unknown> {
  return fetchJSON(`/progress/${albumId}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
}

/** 媒体文件 URL:相对应用部署位置(…/tu-xxx/files/audio/xiyouji/001.mp3) */
export function fileUrl(album: Pick<AlbumSummary, "type" | "id">, file: string): string {
  return new URL(`files/${album.type}/${album.id}/${file}`, document.baseURI).toString();
}
