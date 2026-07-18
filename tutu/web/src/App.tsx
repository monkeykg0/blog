import { HashRouter, Route, Routes, useLocation } from "react-router-dom";
import MiniPlayer from "./components/MiniPlayer";
import AlbumPage from "./pages/AlbumPage";
import Shelf from "./pages/Shelf";

// HashRouter:应用部署在隐秘路径下,免服务器 SPA 重写配置
export default function App() {
  return (
    <HashRouter>
      <Routes>
        <Route path="/" element={<Shelf />} />
        <Route path="/album/:id" element={<AlbumPage />} />
      </Routes>
      <MiniPlayerOnShelf />
    </HashRouter>
  );
}

/** 迷你播放条只在书架页出现(播放页自带完整控制) */
function MiniPlayerOnShelf() {
  const { pathname } = useLocation();
  if (pathname !== "/") return null;
  return <MiniPlayer />;
}
