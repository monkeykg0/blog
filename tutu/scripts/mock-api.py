#!/usr/bin/env python3
"""本地开发 mock:模拟 media-api(无需 MySQL)。

用法: python3 scripts/mock-api.py   # 监听 127.0.0.1:8081
配合: cd media-library && python3 -m http.server 8899   # files
      cd web && pnpm dev                                 # 前端
进度存内存,重启即清零(仅供开发)。
"""
import json
import re
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent / "media-library"
progress: dict[str, bytes] = {}


def albums():
    out = []
    for f in sorted(ROOT.glob("*/*/album.json")):
        out.append(json.loads(f.read_text(encoding="utf-8")))
    return out


class H(BaseHTTPRequestHandler):
    def _json(self, obj, code=200):
        body = json.dumps(obj, ensure_ascii=False).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path == "/api/media/library":
            summary = [{k: a.get(k) for k in
                        ("id", "type", "title", "artist", "cover",
                         "trackCount", "totalDuration", "updatedAt")}
                       for a in albums()]
            return self._json({"albums": summary})
        if m := re.match(r"^/api/media/album/([a-z0-9-]+)$", self.path):
            for a in albums():
                if a["id"] == m.group(1):
                    return self._json(a)
            return self._json({"error": "not found"}, 404)
        if m := re.match(r"^/api/media/progress/([a-z0-9-]+)$", self.path):
            raw = progress.get(m.group(1))
            return self._json({"data": json.loads(raw) if raw else None,
                               "updatedAt": None})
        self._json({"error": "not found"}, 404)

    def do_PUT(self):
        if m := re.match(r"^/api/media/progress/([a-z0-9-]+)$", self.path):
            n = int(self.headers.get("Content-Length", 0))
            progress[m.group(1)] = self.rfile.read(n)
            return self._json({"status": "ok"})
        self._json({"error": "not found"}, 404)

    def log_message(self, *_):
        pass


if __name__ == "__main__":
    print("mock media-api → http://127.0.0.1:8081")
    HTTPServer(("127.0.0.1", 8081), H).serve_forever()
