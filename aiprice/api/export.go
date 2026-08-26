// 导出静态站构建所需的 JSON。
//
// 站点是 Astro 静态预渲染（1300+ 页要靠搜索引擎），所以构建期需要一份完整快照。
// 导出物：
//
//	meta.json              导出时间、数据日期、汇率来源
//	storefronts.json       地区表（含中英文名、旗帜、币种）
//	products.json          产品与主力档位
//	prices/{product}.json  该产品全部档位 × 全部地区的价格
//
// 只导出 parse_status='ok' 的数据。unclassified / unknown_sku 不进站点——
// 宁可少一条，不能显示错的（调研报告 §5.1）。
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ExportMeta 是站点页脚要展示的数据新鲜度信息。
type ExportMeta struct {
	DataDate    string `json:"dataDate"`    // 价格快照日期
	FXDate      string `json:"fxDate"`      // 汇率日期
	FXSource    string `json:"fxSource"`    //
	GeneratedAt string `json:"generatedAt"` // 本次导出时间（RFC3339）
	Storefronts int    `json:"storefronts"` // 地区总数
	Products    int    `json:"products"`
	// CNYPerUSD 是快照当日的美元兑人民币中间价。站点展示人民币时用它换算，
	// 必须用快照当天的汇率，不能用构建当天的（调研报告 §3.5）。
	CNYPerUSD float64 `json:"cnyPerUsd"`
	// PriceCount 是本次导出的有效价格条数。
	PriceCount int `json:"priceCount"`
}

// PriceRow 是一个地区的一个档位的价格。
type PriceRow struct {
	Storefront string  `json:"storefront"`
	Currency   string  `json:"currency"`
	RawPrice   string  `json:"rawPrice"` // Apple 原始展示串，页面上原样显示更可信
	Amount     float64 `json:"amount"`   // 本地币种金额
	USD        float64 `json:"usd"`
}

// TierPrices 是一个档位在全球的价格。
type TierPrices struct {
	Tier   string     `json:"tier"`
	TierEn string     `json:"tierEn"`
	TierZh string     `json:"tierZh"`
	Period string     `json:"period"`
	Rows   []PriceRow `json:"rows"` // 按美元价升序
}

// Export 把最新（或指定日期）的数据导出成 JSON。
func Export(store *Store, outDir, date string) error {
	if date == "" {
		latest, err := store.LatestDate()
		if err != nil {
			return err
		}
		if latest == "" {
			return fmt.Errorf("库里还没有任何 ok 的价格数据，先跑一次 `aiprice crawl`")
		}
		date = latest
	}

	if err := os.MkdirAll(filepath.Join(outDir, "prices"), 0o755); err != nil {
		return err
	}

	fxDate, fxSource := "", ""
	_ = store.db.QueryRow(
		`SELECT date, source FROM fx_rates WHERE date <= ? ORDER BY date DESC LIMIT 1`, date).
		Scan(&fxDate, &fxSource)
	if len(fxDate) >= 10 {
		fxDate = fxDate[:10]
	}

	// 快照当日的人民币汇率，供站点换算展示
	var cnyPerUSD float64
	if err := store.db.QueryRow(
		`SELECT rate FROM fx_rates WHERE date = ? AND base = 'USD' AND currency = 'CNY'`,
		fxDate).Scan(&cnyPerUSD); err != nil {
		return fmt.Errorf("取不到 %s 的 USD/CNY 汇率，站点无法展示人民币: %w", fxDate, err)
	}

	// 地区表
	if err := writeJSON(filepath.Join(outDir, "storefronts.json"), Storefronts()); err != nil {
		return err
	}

	// 产品与主力档位
	type productOut struct {
		Product
		Tiers []SKURule `json:"tiers"`
	}
	var products []productOut
	for _, p := range Products {
		products = append(products, productOut{Product: p, Tiers: FeaturedTiers(p.ID)})
	}
	if err := writeJSON(filepath.Join(outDir, "products.json"), products); err != nil {
		return err
	}

	// 各产品价格
	totalRows := 0
	for _, p := range Products {
		tiers, n, err := exportProduct(store, p.ID, date)
		if err != nil {
			return fmt.Errorf("导出 %s 失败: %w", p.ID, err)
		}
		totalRows += n
		path := filepath.Join(outDir, "prices", p.ID+".json")
		if err := writeJSON(path, tiers); err != nil {
			return err
		}
		log.Printf("%-8s %d 个档位 / %d 条价格", p.ID, len(tiers), n)
	}

	meta := ExportMeta{
		DataDate: date, FXDate: fxDate, FXSource: fxSource,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Storefronts: len(Storefronts()), Products: len(Products),
		CNYPerUSD: cnyPerUSD, PriceCount: totalRows,
	}
	if err := writeJSON(filepath.Join(outDir, "meta.json"), meta); err != nil {
		return err
	}

	log.Printf("✅ 导出完成: %s（数据日期 %s，共 %d 条价格）", outDir, date, totalRows)
	return nil
}

func exportProduct(store *Store, productID, date string) ([]TierPrices, int, error) {
	rows, err := store.db.Query(`
		SELECT tier, period, storefront, currency, raw_price, amount_minor, usd_micro
		FROM price_snapshots
		WHERE product_id = ? AND date = ? AND parse_status = 'ok'
		ORDER BY tier, period, usd_micro`, productID, date)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	grouped := map[string]*TierPrices{}
	count := 0
	for rows.Next() {
		var tier, period, sf, currency, raw string
		var minor, usdMicro int64
		if err := rows.Scan(&tier, &period, &sf, &currency, &raw, &minor, &usdMicro); err != nil {
			return nil, 0, err
		}
		key := tier + "|" + period
		tp, ok := grouped[key]
		if !ok {
			tp = &TierPrices{Tier: tier, Period: period}
			if r := findRule(productID, tier, period); r != nil {
				tp.TierEn, tp.TierZh = r.TierEn, r.TierZh
			}
			grouped[key] = tp
		}
		// 同一地区同一档位可能有多条（促销价等），保留最便宜的一条
		if i := indexOfStorefront(tp.Rows, sf); i >= 0 {
			if float64(usdMicro)/1e6 < tp.Rows[i].USD {
				tp.Rows[i] = newPriceRow(sf, currency, raw, minor, usdMicro)
			}
			continue
		}
		tp.Rows = append(tp.Rows, newPriceRow(sf, currency, raw, minor, usdMicro))
		count++
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	out := make([]TierPrices, 0, len(grouped))
	for _, tp := range grouped {
		sort.Slice(tp.Rows, func(i, j int) bool { return tp.Rows[i].USD < tp.Rows[j].USD })
		out = append(out, *tp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Tier != out[j].Tier {
			return out[i].Tier < out[j].Tier
		}
		return out[i].Period < out[j].Period
	})
	return out, count, nil
}

func newPriceRow(sf, currency, raw string, minor, usdMicro int64) PriceRow {
	exp := CurrencyExponent(currency)
	div := 1.0
	for i := 0; i < exp; i++ {
		div *= 10
	}
	return PriceRow{
		Storefront: sf, Currency: currency, RawPrice: raw,
		Amount: float64(minor) / div,
		USD:    float64(usdMicro) / 1e6,
	}
}

func indexOfStorefront(rows []PriceRow, sf string) int {
	for i := range rows {
		if rows[i].Storefront == sf {
			return i
		}
	}
	return -1
}

func findRule(productID, tier, period string) *SKURule {
	for i := range SKURules {
		r := &SKURules[i]
		if r.ProductID == productID && r.Tier == tier && r.Period == period {
			return r
		}
	}
	return nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
