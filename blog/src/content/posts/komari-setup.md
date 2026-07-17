---
title: 用 Komari 给手里的 4 台小鸡装个统一监控：安装、避坑与主题美化
published: 2026-07-17T18:40:00+08:00
description: 一台面板 + N 个 Agent，把散落在德国、美国、日本的 VPS 全部收进一个页面。记录 Komari 在 1Panel 上的完整搭建过程：反代 WebSocket 配置、延迟监测、以及把面板换成好看的 emerald 主题。
tags: [Komari, 服务器监控, 自托管, 1Panel]
category: 技术
draft: false
---

手里的 VPS 慢慢攒到了 4 台：德国的 netcup 主力机、两台美国小鸡、一台日本小鸡。以前想看某台的负载得挨个 SSH 上去 `htop`，机器一多就烦了。今天花了一个下午，用 [Komari](https://github.com/komari-monitor/komari) 把它们全部收进了一个监控页面，顺便折腾了主题美化。整个过程记录如下。

## 为什么选 Komari

Komari 是一个 Go 写的轻量自托管服务器监控，模式非常简单：

```
美国小鸡 A ── agent ─┐
美国小鸡 B ── agent ─┼─ WebSocket ──> Komari 面板（装在其中一台上）
日本小鸡   ── agent ─┤
德国主力机 ── agent ─┘
```

- **面板只装一台**，其他机器各跑一个几 MB 内存占用的 agent，通过 WebSocket 实时上报
- 面板就一个 Docker 容器 + 一个数据目录，没有一堆依赖
- 自带延迟/丢包监测（Ping 任务）、流量统计、到期提醒
- 支持自定义主题，社区主题质量很高（后面细说）

## 一、安装面板

我的主力机装了 1Panel，应用商店里直接就能搜到 `komari`，一键安装完事，默认服务端口 25774。没有 1Panel 的话用官方 docker-compose 也一样简单：

```yaml
services:
  komari:
    image: ghcr.io/komari-monitor/komari:latest
    container_name: komari
    ports:
      - "127.0.0.1:25774:25774"
    volumes:
      - ./data:/app/data
    restart: unless-stopped
```

初始管理员账号密码在容器日志里，搜 `admin` 就能找到，第一次登录后记得改掉。

**安全要点：端口不要直接暴露公网。** 注意上面 compose 里写的是 `127.0.0.1:25774:25774`——Docker 的端口映射会绕过 ufw 防火墙，如果绑 `0.0.0.0`，你以为防火墙挡住了，其实全世界都能访问。1Panel 商店安装的话，把应用参数里的「端口外部访问」取消勾选，效果相同。对外访问全部走下一步的反向代理。

## 二、反向代理 + HTTPS

给面板配一个子域名（比如 `komari.example.com`），走 OpenResty/Nginx 反代到 `127.0.0.1:25774`。我的域名托管在 Cloudflare，加一条 A 记录并开启橙云代理，源站证书用 CF 的 Origin 通配符证书。

**踩坑 1**：1Panel 创建反向代理网站时，「代理地址」栏**不要带 `http://` 前缀**，直接填 `127.0.0.1:25774`。带了前缀会生成 `proxy_pass http://http://...` 这种配置，nginx 校验直接报错 `invalid port in upstream`。

**关键配置：WebSocket 必须打通**，否则 agent 连不上、页面数据也不会动。反代配置里确认有这几行：

```nginx
proxy_http_version 1.1;
proxy_set_header Upgrade $http_upgrade;
proxy_set_header Connection $http_connection;
proxy_set_header Host $host;
proxy_read_timeout 300s;
```

Cloudflare 免费版原生支持 WebSocket，橙云套在前面完全没问题，agent 流量走 CF 回源，源站防火墙只放行 CF 网段的安全策略也不用动。

## 三、给每台机器装 Agent

面板后台「节点管理」里逐台添加节点，每个节点会生成一条自带地址和 token 的一键安装命令，复制到对应机器上执行即可：

```bash
wget -qO- https://raw.githubusercontent.com/komari-monitor/komari-agent/refs/heads/main/install.sh | bash -s -- -e https://komari.example.com --auto-discovery <token>
```

agent 会注册成 systemd 服务，开机自启。别忘了面板所在的机器自己也要装一个，不然它监控不到自己。

**踩坑 2**：官方文档给的命令带 `sudo`，很多小鸡默认就是 root 登录且根本没装 sudo，会报 `sudo: command not found`——把 `sudo` 去掉直接跑就行。

## 四、补齐数据：延迟监测、价格与流量

装完 agent 只有 CPU/内存/流量这些基础数据，面板好看的部分要自己配：

**Ping 任务**（后台「延迟监测」）：添加监测目标并分配给所有节点，我加了三条，间隔都是 60 秒：

| 任务 | 类型 | 目标 | 用途 |
|---|---|---|---|
| Cloudflare | ICMP | `1.1.1.1` | 国际基准延迟 |
| 阿里DNS | ICMP | `223.5.5.5` | 回国延迟（最有参考价值） |
| 博客 | HTTP | 我的博客地址 | 顺便从美/日/德三个方向拨测博客可用性 |

**节点信息**：每个节点编辑页里填上价格、账单周期、到期时间、流量额度和地区标签。填完之后面板就能算出剩余价值、到期倒计时、流量使用百分比——量化一下自己在小鸡上花了多少钱，还是挺有冲击力的。

## 五、主题美化

Komari 支持上传自定义主题（后台 → 设置 → 主题，传 zip 即可，随时切换）。我先后试了两个，风格差异不小：

- **[purcarte-plus](https://github.com/YoungYannick/komari-theme-purcarte-plus)**：磨砂玻璃风，60+ 配置项，访客欢迎气泡、资产统计、3D 地球都有。但卡片正面不显示延迟彩条，延迟图要点进详情或「延迟总览」页看。
- **[emerald](https://github.com/Tokinx/komari-theme-emerald)**：Vue 3 + Shadcn/UI，卡片正面直接带延迟/丢包彩条，信息密度高，整体更清爽。

一开始我用 purcarte-plus，后来发现自己最想一眼看到的就是各机器的延迟和丢包，最终切到了 **emerald**。背景图的话，主题设置里填一个图片 URL 就行——建议把图传到自己的站点目录用自己的域名引用，别依赖外链图床，图片压到几百 KB 以内，访客首屏加载会舒服很多。

## 成果

最终效果：一个页面看全 4 台机器的实时负载、各线路延迟丢包、流量消耗和到期时间，手机上打开也很流畅。整个过程不算踩坑一小时就能搞定，踩了坑也就一下午。😄

👉 我的监控面板：**[https://komari.monkeykgai.com](https://komari.monkeykgai.com)**，欢迎围观。
