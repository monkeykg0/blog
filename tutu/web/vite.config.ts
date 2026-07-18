import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { VitePWA } from "vite-plugin-pwa";

// base 用相对路径:应用部署在隐秘路径 /tu-<随机串>/ 下,构建产物与路径无关
export default defineConfig({
  base: "./",
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      registerType: "autoUpdate",
      includeAssets: ["icons/*"],
      manifest: {
        name: "兔兔听书屋",
        short_name: "兔兔听书",
        description: "小朋友的听书小屋",
        lang: "zh-CN",
        start_url: ".",
        scope: ".",
        display: "standalone",
        background_color: "#FFF6EA",
        theme_color: "#FF8A3D",
        icons: [
          { src: "icons/icon-192.png", sizes: "192x192", type: "image/png" },
          { src: "icons/icon-512.png", sizes: "512x512", type: "image/png" },
          { src: "icons/icon-512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
        ],
      },
      workbox: {
        // 只预缓存构建产物;音频(大)与 /api(实时)都不缓存
        globPatterns: ["**/*.{js,css,html,svg,png,ico}"],
        navigateFallback: null,
      },
    }),
  ],
  server: {
    proxy: {
      // 本地开发:api → 本地 Go 服务;files → python -m http.server 8899(在 media-library 目录)
      "/api": "http://127.0.0.1:8081",
      "/files": {
        target: "http://127.0.0.1:8899",
        rewrite: (p) => p.replace(/^\/files/, ""),
      },
    },
  },
});
