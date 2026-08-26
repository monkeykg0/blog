# aiprice · AI 订阅全球价格观测

每日抓取 ChatGPT / Claude / Gemini / Grok 在 **158 个 App Store 地区**的订阅价格，
折算美元与人民币，记录历史变价。

背景调研、实测数据与设计决策见 [`../AI订阅全球价格站可行性调研.md`](../AI订阅全球价格站可行性调研.md)。

```
访客 → Cloudflare → OpenResty (443) → aiprice.monkeykgai.com → web/dist 静态文件
                                                                      ↑
                       systemd timer 每日 03:30 → aiprice crawl → MySQL → aiprice export
```

## 目录

| 目录 | 说明 |
|---|---|
| `api/` | Go 抓取程序 `aiprice`。抓取、解析、归一化、落库、导出 JSON |
| `web/` | Astro 静态站（中英双语，359 页），构建期读 `api export` 产出的 JSON |
| `design/` | 界面设计稿（`.dc.html` artboards），改完重新 seed 即可更新设计画布 |
| `deploy/` | 部署脚本、systemd service/timer、建库 SQL |

## 命令

```bash
cd api
go test ./...                      # 价格解析器单测（78 种格式的回归用例）
go run . check                     # 自检：地区表 / 汇率源 / 出口 IP
go run . crawl -dry-run -limit 12  # 抓 12 个地区，不落库，验证 SKU 规则
go run . crawl                     # 全量抓取并落库（约 40 分钟）
go run . export -out ../web/src/data
```

⚠️ **`crawl` 和 `check` 在中国大陆 IP 上会失败**。Apple 对中国大陆 IP 强制锁 `/cn/` 商店，
无论请求哪个地区码都会跳转到 Today 页。程序会明确报错而不是返回错数据。
本地开发请走代理，生产跑在德国 VPS 上。

## 两个关键设计

**价格解析不看货币符号。** 同样是 `$`，美国是 `$19.99`，哥伦比亚是 `$ 99.900,00`
（点千分位、逗号小数点）。币种由「地区 → ISO 币种」映射给定，小数位数查 ISO 4217 exponent 表。
实测单个 SKU 有 78 种格式，`api/price_test.go` 是回归防线，改解析器前先看它。

**SKU 白名单需要人工维护。** Apple 只给「名称 + 价格」，不给订阅周期——
美区 `ChatGPT Plus` 同时是 $19.99 的月付和 $200.00 的年付，名字一模一样。
归一化靠「名称 + 美元价格区间」联合判定，规则在 `api/catalog.go`。
每次 `crawl` 结束会打印 `unknown_sku` 和 `unclassified` 汇总，看到告警就去补规则。

价格落在所有已知区间之外时**不会发布**，只记为 `unclassified` 等人工确认。
这条规则在实测中抓出过两轮解析 bug（匈牙利/哥伦比亚算大 100 倍、印尼 `Rp 1,999juta` 算大 1000 倍），
不要为了「少点告警」把它关掉。

## 数据表

| 表 | 说明 |
|---|---|
| `price_snapshots` | 每日价格快照，**只追加不覆盖**。保留 Apple 原始价格字符串 `raw_price` |
| `fx_rates` | 每日汇率存档。历史价格配历史汇率，不用当前汇率回算 |
| `crawl_runs` | 每次抓取的成功/未上架/失败计数，错误率超 5% 判定 aborted 且不发布数据 |

## 历史与变价

`api/history.go` 汇总两样东西给站点：每个档位**逐日的全球最低价**（画趋势线），
以及**变价事件**（某地区某档位的本地标价相对上次抓取变了）。

⚠️ 变价判定比较的是 `amount_minor`（本地货币），**不是折算后的美元**。
汇率天天在动，按美元比会把每一次汇率波动都误报成「厂商调价」。

只有一天数据时曲线画不出来，历史页会显示「积累中」并展示当天的起点值——
这是设计好的状态，不是 bug。第二天起自动出线。

## 前端

```bash
cd web
npm install
npm run dev            # 本地预览（用 src/data 里已有的数据）
npm run build          # 构建到 dist/
```

部署：`./deploy/deploy-web.sh`（会先从服务器导出最新数据再构建），
只改样式不用重新取数时加 `--skip-export`。

**排名一律用竞赛排名**（比它严格便宜的地区数 + 1，并列同名次）。
不能用排序后的数组下标——158 个地区只有 42 个不同价位，光 `$19.99` 就有 86 个地区，
下标名次在并列里完全是任意的（同一个美国能排出第 11 也能排出第 90，取决于排序稳定性）。
`web/src/lib/data.ts` 里的 `withRanks` / `rankOf` 是唯一正确入口。

## 服务器初始化

见 `deploy/`：`init.sql` 建库，`env.example` 抄成 `/opt/aiprice/env`（`chmod 600`），
然后 `./deploy/deploy-api.sh`。
