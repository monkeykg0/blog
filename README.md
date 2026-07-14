# 博客 + 访问统计

Astro 静态博客 + Go 统计后端，部署在 netcup VPS（1Panel / OpenResty / MySQL / Redis），前置 Cloudflare。

```
访客 → Cloudflare → OpenResty (443)
                      ├── /       → blog/dist 静态文件
                      └── /api/*  → blog-stats (127.0.0.1:8080) → Redis + MySQL
```

## 目录

| 目录 | 说明 |
|---|---|
| `blog/` | Astro 博客。`ViewBeacon.astro` 全站上报浏览，`PageViews.astro` 展示文章阅读量 |
| `server/` | Go 后端 `blog-stats`。`POST /api/view` 计数，`GET /api/views?path=` 查询 PV/UV |
| `deploy/` | 部署脚本、systemd 服务、OpenResty 反代配置、MySQL 初始化 SQL |

## 统计逻辑

- 前端 `sendBeacon` 上报（只有真实浏览器执行 JS），后端再过滤爬虫 UA
- 真实 IP 取 `CF-Connecting-IP` 头
- UV：Redis `SETNX`（IP+UA 哈希）当日去重；PV/UV 按日落 MySQL `page_views_daily` 表
- 单 IP 限流：10 秒 30 次

## 首次部署步骤

1. **MySQL**：在 1Panel 执行 `deploy/init.sql`（改密码），建 `blog` 库和用户
2. **后端配置**：服务器上创建 `/opt/blog-stats/env`（参考 `deploy/env.example`，`chmod 600`）
3. **部署后端**：`./deploy/deploy-server.sh`
4. **建站点**：1Panel → 网站 → 创建静态网站（绑定域名），把站点 server 块加入 `deploy/openresty-api.conf` 的 `/api/` 反代
5. **部署博客**：`WEB_ROOT=<1Panel站点目录> ./deploy/deploy-blog.sh`
6. **Cloudflare**：
   - DNS 记录开启代理（橙色云）
   - SSL 模式 Full (Strict)，源站用 Cloudflare Origin Certificate（在 1Panel 站点里导入）
   - Cache Rule：`/api/*` 设为 Bypass
7. **安全收口**：
   - ufw 的 80/443 只放行 [Cloudflare IP 段](https://www.cloudflare.com/ips/)（防止直连源站伪造 `CF-Connecting-IP` 刷量）
   - ⚠️ 1Panel 里 Redis (6379) / MySQL (3306) 是 Docker 端口映射，**会绕过 ufw**。到 1Panel 应用参数里确认端口只绑定 127.0.0.1，或关闭外部访问

## 日常更新

- 写文章：`blog/src/content/blog/` 下加 Markdown，然后 `./deploy/deploy-blog.sh`
- 改后端：`./deploy/deploy-server.sh`（自动编译、上传、重启、健康检查）
