// 产品目录与 SKU 白名单。
//
// 这是全项目唯一需要持续人工维护的地方（调研报告 §5.3）。原因：
// Apple 只给「名称 + 价格」，**不给订阅周期**。而 OpenAI 和 Google 的命名完全不规范——
// 美区 "ChatGPT Plus" 同时是 $19.99 的月付和 $200.00 的年付，名字一模一样。
//
// 所以归一化靠「展示名 + 美元价格区间」两个条件联合判定：
// 先按名称找到候选，再用折算后的美元价落到哪个区间来定周期。
//
// 维护提醒：
//   - 区间要留足冗余（各地区购买力定价差异可达 50%），但不能重叠到别的档位上；
//   - 厂商调价后如果落到区间外，会被标成 unclassified 而不是静默出错；
//   - `aiprice crawl` 结束时会打印 unclassified 汇总，看到了就来这里加规则。
package main

import (
	"regexp"
	"strings"
)

// Product 是一个可比价的产品。
type Product struct {
	ID         string `json:"id"` // 内部 id
	NameEn     string `json:"nameEn"`
	NameZh     string `json:"nameZh"`
	Vendor     string `json:"vendor"`
	AppStoreID int64  `json:"appStoreId"`
	BundleID   string `json:"bundleId"`
}

// Products 是当前纳入抓取的产品。加产品只需要在这里加一行 + 补 SKU 规则，
// 抓取和解析逻辑完全复用（调研报告 §10.4）。
var Products = []Product{
	{ID: "chatgpt", NameEn: "ChatGPT", NameZh: "ChatGPT", Vendor: "OpenAI", AppStoreID: 6448311069, BundleID: "com.openai.chat"},
	{ID: "claude", NameEn: "Claude", NameZh: "Claude", Vendor: "Anthropic", AppStoreID: 6473753684, BundleID: "com.anthropic.claude"},
	{ID: "gemini", NameEn: "Google Gemini", NameZh: "Google Gemini", Vendor: "Google", AppStoreID: 6477489729, BundleID: "com.google.gemini"},
	{ID: "grok", NameEn: "Grok", NameZh: "Grok", Vendor: "xAI", AppStoreID: 6670324846, BundleID: "ai.x.GrokApp"},
}

// 订阅周期。
const (
	PeriodMonthly = "monthly"
	PeriodAnnual  = "annual"
	PeriodOneOff  = "oneoff" // 加油包 / 点数，一次性
)

// SKURule 把 Apple 的展示名归一化成一个档位。
type SKURule struct {
	ProductID string `json:"productId"`
	// Aliases 是 Apple 在各地区展示的名称。绝大多数地区名称统一，
	// 但 Google 会本地化存储档位（法语区 "100 Go" ≠ "100 GB"），所以留了别名。
	Aliases []string `json:"-"`
	Tier    string   `json:"tier"` // 归一化档位 id，跨地区、跨语言稳定
	TierEn  string   `json:"tierEn"`
	TierZh  string   `json:"tierZh"`
	Period  string   `json:"period"`
	// MinUSD/MaxUSD 是折算成美元后的合理区间，用来在同名 SKU 之间区分周期，
	// 同时充当异常价格的兜底校验（调研报告 §5.1 第 4 条）。
	MinUSD float64 `json:"-"`
	MaxUSD float64 `json:"-"`
	// Featured 表示是否是该产品的主力档位（首页和排行页只展示主力档）。
	Featured bool `json:"featured"`
}

// 区间取值方法：先跑 `crawl -dry-run` 拿到 158 个地区的实测分布，再在观测到的
// 上下限外各留约 40% 冗余。**同一个别名下的月付区间和年付区间绝不能重叠**，
// 否则周期会判错；不同别名之间可以重叠（Classify 先按名称过滤）。
// 下面括号里是 2026-08-26 的实测区间。
var SKURules = []SKURule{
	// ---- ChatGPT ----
	{ProductID: "chatgpt", Aliases: []string{"ChatGPT Go"}, Tier: "go", TierEn: "ChatGPT Go", TierZh: "ChatGPT Go", Period: PeriodMonthly, MinUSD: 2, MaxUSD: 16, Featured: true}, // 实测 4.18–10.62
	{ProductID: "chatgpt", Aliases: []string{"ChatGPT Plus"}, Tier: "plus", TierEn: "ChatGPT Plus", TierZh: "ChatGPT Plus", Period: PeriodMonthly, MinUSD: 8, MaxUSD: 60, Featured: true}, // 实测 16.20–32.62
	{ProductID: "chatgpt", Aliases: []string{"ChatGPT Plus"}, Tier: "plus", TierEn: "ChatGPT Plus", TierZh: "ChatGPT Plus", Period: PeriodAnnual, MinUSD: 100, MaxUSD: 500},                        // 实测 161.95–326.45
	{ProductID: "chatgpt", Aliases: []string{"ChatGPT Pro 5x"}, Tier: "pro5x", TierEn: "ChatGPT Pro 5x", TierZh: "ChatGPT Pro 5x", Period: PeriodMonthly, MinUSD: 60, MaxUSD: 200, Featured: true},  // 实测 99.18–130.70
	{ProductID: "chatgpt", Aliases: []string{"ChatGPT Pro 20x"}, Tier: "pro20x", TierEn: "ChatGPT Pro 20x", TierZh: "ChatGPT Pro 20x", Period: PeriodMonthly, MinUSD: 100, MaxUSD: 500, Featured: true}, // 实测 161.95–326.45

	// ---- Claude（四家里命名最规范，名字里自带周期）----
	{ProductID: "claude", Aliases: []string{"Claude Pro - Monthly"}, Tier: "pro", TierEn: "Claude Pro", TierZh: "Claude Pro", Period: PeriodMonthly, MinUSD: 8, MaxUSD: 60, Featured: true},               // 实测 17.66–32.62
	{ProductID: "claude", Aliases: []string{"Claude Pro - Annual"}, Tier: "pro", TierEn: "Claude Pro", TierZh: "Claude Pro", Period: PeriodAnnual, MinUSD: 120, MaxUSD: 500},                             // 实测 198.32–326.45
	{ProductID: "claude", Aliases: []string{"Claude Max 5x - Monthly"}, Tier: "max5x", TierEn: "Claude Max 5x", TierZh: "Claude Max 5x", Period: PeriodMonthly, MinUSD: 70, MaxUSD: 300, Featured: true},  // 实测 112.84–195.86
	{ProductID: "claude", Aliases: []string{"Claude Max 20x - Monthly"}, Tier: "max20x", TierEn: "Claude Max 20x", TierZh: "Claude Max 20x", Period: PeriodMonthly, MinUSD: 150, MaxUSD: 600, Featured: true}, // 实测 225.75–391.75

	// ---- Gemini（四家里最乱：同名重复、存储档位按地区变、法语区把 GB 写成 Go）----
	{ProductID: "gemini", Aliases: []string{"Google AI Plus (400 GB)", "Google AI Plus (400 Go)", "Google AI Plus (200GB)"}, Tier: "ai_plus", TierEn: "Google AI Plus", TierZh: "Google AI Plus", Period: PeriodMonthly, MinUSD: 2, MaxUSD: 16, Featured: true}, // 实测 4.16–7.27
	{ProductID: "gemini", Aliases: []string{"Google AI Plus (400 GB)", "Google AI Plus (400 Go)", "Google AI Plus (200GB)"}, Tier: "ai_plus", TierEn: "Google AI Plus", TierZh: "Google AI Plus", Period: PeriodAnnual, MinUSD: 25, MaxUSD: 120},                // 实测 41.92–63.80
	{ProductID: "gemini", Aliases: []string{"Google AI Pro (5 TB)", "Google AI Pro (5 To)"}, Tier: "ai_pro", TierEn: "Google AI Pro", TierZh: "Google AI Pro", Period: PeriodMonthly, MinUSD: 7, MaxUSD: 50, Featured: true},                                    // 实测 11.22–28.37
	{ProductID: "gemini", Aliases: []string{"Google AI Pro (5 TB)", "Google AI Pro (5 To)"}, Tier: "ai_pro", TierEn: "Google AI Pro", TierZh: "Google AI Pro", Period: PeriodAnnual, MinUSD: 120, MaxUSD: 400},                                                  // 实测 199.99–244.23
	{ProductID: "gemini", Aliases: []string{"Google AI Ultra (30 TB)", "Google AI Ultra (30 To)"}, Tier: "ai_ultra", TierEn: "Google AI Ultra", TierZh: "Google AI Ultra", Period: PeriodMonthly, MinUSD: 100, MaxUSD: 450, Featured: true},                     // 实测 149.99–291.73
	// 只在少数地区出现的存储变体，收下但不进主对比
	{ProductID: "gemini", Aliases: []string{"Google AI Plus (2 TB)"}, Tier: "ai_plus_2tb", TierEn: "Google AI Plus (2 TB)", TierZh: "Google AI Plus (2 TB)", Period: PeriodMonthly, MinUSD: 4, MaxUSD: 20},   // 实测 7.48–11.74
	{ProductID: "gemini", Aliases: []string{"Google AI Plus (2 TB)"}, Tier: "ai_plus_2tb", TierEn: "Google AI Plus (2 TB)", TierZh: "Google AI Plus (2 TB)", Period: PeriodAnnual, MinUSD: 60, MaxUSD: 200},  // 实测 99.99
	{ProductID: "gemini", Aliases: []string{"Google AI Pro (10 TB)"}, Tier: "ai_pro_10tb", TierEn: "Google AI Pro (10 TB)", TierZh: "Google AI Pro (10 TB)", Period: PeriodMonthly, MinUSD: 25, MaxUSD: 100}, // 实测 49.99
	{ProductID: "gemini", Aliases: []string{"Google AI Ultra (20 TB)"}, Tier: "ai_ultra_20tb", TierEn: "Google AI Ultra (20 TB)", TierZh: "Google AI Ultra (20 TB)", Period: PeriodMonthly, MinUSD: 50, MaxUSD: 200}, // 实测 99.99

	// ---- Grok（和 OpenAI 一样，月付年付同名，只能靠价格区间分）----
	{ProductID: "grok", Aliases: []string{"SuperGrok Lite"}, Tier: "lite", TierEn: "SuperGrok Lite", TierZh: "SuperGrok Lite", Period: PeriodMonthly, MinUSD: 5, MaxUSD: 30, Featured: true}, // 实测 9.42–16.29
	{ProductID: "grok", Aliases: []string{"SuperGrok Lite"}, Tier: "lite", TierEn: "SuperGrok Lite", TierZh: "SuperGrok Lite", Period: PeriodAnnual, MinUSD: 60, MaxUSD: 260},                // 实测 100.00–163.21
	{ProductID: "grok", Aliases: []string{"SuperGrok"}, Tier: "supergrok", TierEn: "SuperGrok", TierZh: "SuperGrok", Period: PeriodMonthly, MinUSD: 15, MaxUSD: 70, Featured: true},          // 实测 27.02–42.82
	{ProductID: "grok", Aliases: []string{"SuperGrok"}, Tier: "supergrok", TierEn: "SuperGrok", TierZh: "SuperGrok", Period: PeriodAnnual, MinUSD: 150, MaxUSD: 700},                         // 实测 243.01–428.21
	{ProductID: "grok", Aliases: []string{"SuperGrok Plus"}, Tier: "plus", TierEn: "SuperGrok Plus", TierZh: "SuperGrok Plus", Period: PeriodMonthly, MinUSD: 50, MaxUSD: 260, Featured: true}, // 实测 89.72–163.21
	{ProductID: "grok", Aliases: []string{"SuperGrok Plus"}, Tier: "plus", TierEn: "SuperGrok Plus", TierZh: "SuperGrok Plus", Period: PeriodAnnual, MinUSD: 600, MaxUSD: 2000},                // 实测 1000.00–1257.82
	{ProductID: "grok", Aliases: []string{"SuperGrok Heavy"}, Tier: "heavy", TierEn: "SuperGrok Heavy", TierZh: "SuperGrok Heavy", Period: PeriodMonthly, MinUSD: 150, MaxUSD: 700, Featured: true}, // 实测 243.01–428.21
}

// 明确不纳入比价的 SKU：一次性点数/加油包，以及 Google One 的纯存储档位
// （后者是网盘容量，不是 AI 订阅）。这些照样落库留档，只是不进站点、
// 也不计入「没见过的 SKU」告警——否则真正的新档位会被淹没在噪音里。
var storageOnlyRe = regexp.MustCompile(`^\d+\s?(GB|TB|Go|To)$`)

// IsIgnoredSKU 判断该展示名是否是有意跳过的品类。
func IsIgnoredSKU(displayName string) bool {
	name := strings.TrimSpace(displayName)
	if storageOnlyRe.MatchString(name) {
		return true
	}
	return strings.Contains(strings.ToLower(name), "credits")
}

// Classify 按「展示名 + 美元价」把一条抓取结果归一化到某个档位。
// 返回 nil 表示没有匹配规则——调用方应记为 unclassified 并保留原始串，
// 绝不能猜，也绝不能直接发布。
func Classify(productID, displayName string, usd float64) *SKURule {
	name := strings.TrimSpace(displayName)
	var nameMatched bool
	for i := range SKURules {
		r := &SKURules[i]
		if r.ProductID != productID || !matchesAlias(r, name) {
			continue
		}
		nameMatched = true
		if usd >= r.MinUSD && usd <= r.MaxUSD {
			return r
		}
	}
	_ = nameMatched
	return nil
}

func matchesAlias(r *SKURule, name string) bool {
	for _, a := range r.Aliases {
		if strings.EqualFold(a, name) {
			return true
		}
	}
	return false
}

// KnownSKUName 判断这个展示名是否在白名单里（不看价格）。
// 用来区分两种 unclassified：名字就没见过（可能是新档位），
// 还是名字认识但价格离谱（很可能是解析出错或厂商大调价）。
func KnownSKUName(productID, displayName string) bool {
	name := strings.TrimSpace(displayName)
	for i := range SKURules {
		if SKURules[i].ProductID == productID && matchesAlias(&SKURules[i], name) {
			return true
		}
	}
	return false
}

// FeaturedTiers 返回某产品的主力档位（保持 SKURules 里的声明顺序）。
func FeaturedTiers(productID string) []SKURule {
	var out []SKURule
	seen := map[string]bool{}
	for _, r := range SKURules {
		if r.ProductID == productID && r.Featured && !seen[r.Tier] {
			seen[r.Tier] = true
			out = append(out, r)
		}
	}
	return out
}

// LookupProduct 按 id 取产品。
func LookupProduct(id string) (Product, bool) {
	for _, p := range Products {
		if p.ID == id {
			return p, true
		}
	}
	return Product{}, false
}
