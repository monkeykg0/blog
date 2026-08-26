// 抓取编排：拉汇率 → 逐地区抓四个产品 → 解析归一化 → 落库。
//
// 实测基线（德国 VPS，单线程，1.2s 间隔）：
//
//	632 次请求 / 零抓取失败 / 零 429 / 约 40 分钟
//	11 次 404 全是「该产品未在该地区上架」，与独立核对的上架数据吻合
//
// 失败策略：单次全量的**真实错误率**超过 5% 就判定为 aborted，不让残缺数据
// 覆盖昨天的好数据（404 不算错误，那是正常的未上架）。
package main

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"sort"
	"time"
)

// CrawlResult 是一次全量抓取的结果汇总。
type CrawlResult struct {
	Date         string
	OK           int
	Absent       int
	Errors       int
	Unclassified []string // 需要人工看的：名字认识但价格离谱
	UnknownSKU   []string // 需要人工看的：没见过的新档位
	Duration     time.Duration
}

// CrawlOptions 控制抓取范围。Limit/Product 只用于开发调试，
// 生产的每日抓取跑全量（两者都留空）。
type CrawlOptions struct {
	DryRun  bool
	Limit   int    // 只抓前 N 个地区，0 表示全部
	Product string // 只抓某个产品，空表示全部
}

// Crawl 跑一次全量抓取。
func Crawl(store *Store, opts CrawlOptions) (*CrawlResult, error) {
	dryRun := opts.DryRun
	start := time.Now()
	fetcher := NewFetcher()

	fx, err := FetchFX(fetcher.Client)
	if err != nil {
		return nil, fmt.Errorf("拉汇率失败，中止抓取（没有汇率就没法做合理性校验）: %w", err)
	}
	log.Printf("汇率: %s，%d 种货币，来源 %s", fx.Date, len(fx.Rates), fx.Source)

	date := time.Now().UTC().Format("2006-01-02")
	res := &CrawlResult{Date: date}

	var runID int64
	if !dryRun {
		if err := store.SaveFX(fx); err != nil {
			return nil, fmt.Errorf("汇率落库失败: %w", err)
		}
		if runID, err = store.StartRun(date); err != nil {
			return nil, err
		}
	}

	unclassified := map[string]int{}
	unknownSKU := map[string]int{}
	var batch []Snapshot

	products := Products
	if opts.Product != "" {
		p, ok := LookupProduct(opts.Product)
		if !ok {
			return nil, fmt.Errorf("没有这个产品: %s", opts.Product)
		}
		products = []Product{p}
	}
	sfList := Storefronts()
	if opts.Limit > 0 && opts.Limit < len(sfList) {
		sfList = sfList[:opts.Limit]
	}

	total := len(products) * len(sfList)
	done := 0

	for _, product := range products {
		for _, sf := range sfList {
			done++
			offers, err := fetcher.Offers(sf.Code, product.AppStoreID)
			switch {
			case err == ErrNotAvailable:
				res.Absent++
				continue
			case err != nil:
				res.Errors++
				log.Printf("[%d/%d] %s/%s 抓取失败: %v", done, total, product.ID, sf.Code, err)
				// 出口 IP 不对的话继续跑没有意义，直接中止
				if isFatalFetchError(err) {
					return nil, err
				}
				continue
			}
			res.OK++

			for _, o := range offers {
				snap := buildSnapshot(date, product, sf, o, fx, unclassified, unknownSKU)
				batch = append(batch, snap)
			}

			if done%40 == 0 {
				log.Printf("[%d/%d] ok=%d absent=%d err=%d 已用时 %s",
					done, total, res.OK, res.Absent, res.Errors, time.Since(start).Round(time.Second))
			}
		}
	}

	res.Duration = time.Since(start)
	res.Unclassified = topKeys(unclassified, 30)
	res.UnknownSKU = topKeys(unknownSKU, 30)

	// 真实错误率（404 不计入）
	attempted := res.OK + res.Errors
	errRate := 0.0
	if attempted > 0 {
		errRate = float64(res.Errors) / float64(attempted)
	}
	status := "success"
	note := fmt.Sprintf("耗时 %s，错误率 %.1f%%，unclassified %d 种，unknown_sku %d 种",
		res.Duration.Round(time.Second), errRate*100, len(res.Unclassified), len(res.UnknownSKU))

	if errRate > 0.05 {
		status = "aborted"
		note = "错误率超过 5%，本次数据不予发布。" + note
	}

	if dryRun {
		log.Printf("[dry-run] 不落库。%s", note)
		return res, nil
	}

	if status == "success" {
		if err := store.SaveSnapshots(batch); err != nil {
			_ = store.FinishRun(runID, "aborted", res.OK, res.Absent, res.Errors, "落库失败: "+err.Error())
			return nil, err
		}
	} else {
		log.Printf("⚠️  %s", note)
	}
	if err := store.FinishRun(runID, status, res.OK, res.Absent, res.Errors, note); err != nil {
		return nil, err
	}
	if status == "aborted" {
		return res, fmt.Errorf("抓取被判定为失败: %s", note)
	}
	return res, nil
}

func buildSnapshot(date string, p Product, sf Storefront, o RawOffer, fx *FXRates,
	unclassified, unknownSKU map[string]int) Snapshot {

	snap := Snapshot{
		Date: date, ProductID: p.ID, Storefront: sf.Code, Currency: sf.Currency,
		DisplayName: o.Name, RawPrice: o.Price,
	}

	minor, err := ParsePrice(o.Price, sf.Currency)
	if err != nil {
		snap.ParseStatus = "parse_error"
		snap.Note = nullString(err.Error())
		return snap
	}
	snap.AmountMinor = sql.NullInt64{Int64: minor, Valid: true}

	amount := float64(minor) / math.Pow10(CurrencyExponent(sf.Currency))
	usd, ok := fx.ToUSD(amount, sf.Currency)
	if !ok {
		snap.ParseStatus = "parse_error"
		snap.Note = nullString("汇率表里没有 " + sf.Currency)
		return snap
	}
	snap.USDMicro = sql.NullInt64{Int64: int64(usd * 1e6), Valid: true}

	rule := Classify(p.ID, o.Name, usd)
	if rule == nil {
		key := fmt.Sprintf("%s / %s", p.ID, o.Name)
		switch {
		case IsIgnoredSKU(o.Name):
			// 点数包和纯存储档位：留档但不进站点，也不告警
			snap.ParseStatus = "ignored"
		case KnownSKUName(p.ID, o.Name):
			// 名字认识但价格落在所有区间之外——要么厂商大调价，要么解析出错。
			// 这正是调研报告 §5.1 里两次 100 倍错误被抓出来的机制。
			snap.ParseStatus = "unclassified"
			snap.Note = nullString(fmt.Sprintf("价格 $%.2f 不在任何已知区间内 (%s %s)", usd, sf.Code, o.Price))
			unclassified[fmt.Sprintf("%s  $%.2f  %s", key, usd, sf.Code)]++
		default:
			snap.ParseStatus = "unknown_sku"
			unknownSKU[key]++
		}
		return snap
	}

	snap.Tier = nullString(rule.Tier)
	snap.Period = nullString(rule.Period)
	snap.ParseStatus = "ok"
	return snap
}

func isFatalFetchError(err error) bool {
	return err != nil && contains(err.Error(), "Today 页")
}

func nullString(s string) sql.NullString {
	if len(s) > 250 {
		s = s[:250]
	}
	return sql.NullString{String: s, Valid: true}
}

func topKeys(m map[string]int, limit int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return keys[i] < keys[j]
	})
	if len(keys) > limit {
		keys = keys[:limit]
	}
	return keys
}
