// 历史序列与变价事件导出。
//
// 站点不需要「每个地区每天的每个价格」——那是几百万行，前端也用不上。
// 只导出两样东西：
//
//   1. 每个档位**每日的全球最低价**（含最低价所在地区），画趋势曲线用；
//   2. **变价事件**：某地区某档位的本地价格相对上一次抓取发生了变化。
//
// ⚠️ 变价判定比较的是 amount_minor（本地货币最小单位），不是折算后的美元。
// 汇率天天在动，用美元比会把每一次汇率波动都误报成「厂商调价」。
// 只有厂商真的改了本地标价，才算一次变价。
package main

import (
	"fmt"
	"sort"
)

// HistoryPoint 是某档位某天的全球最低价。
type HistoryPoint struct {
	Date       string  `json:"date"`
	USD        float64 `json:"usd"`
	Storefront string  `json:"storefront"`
}

// HistorySeries 是一个档位的逐日最低价序列。
type HistorySeries struct {
	ProductID string         `json:"productId"`
	Tier      string         `json:"tier"`
	TierEn    string         `json:"tierEn"`
	TierZh    string         `json:"tierZh"`
	Period    string         `json:"period"`
	Points    []HistoryPoint `json:"points"`
}

// PriceChange 是一次变价事件。
type PriceChange struct {
	Date       string  `json:"date"`
	PrevDate   string  `json:"prevDate"`
	ProductID  string  `json:"productId"`
	Tier       string  `json:"tier"`
	TierEn     string  `json:"tierEn"`
	TierZh     string  `json:"tierZh"`
	Period     string  `json:"period"`
	Storefront string  `json:"storefront"`
	Currency   string  `json:"currency"`
	FromRaw    string  `json:"fromRaw"`
	ToRaw      string  `json:"toRaw"`
	Pct        float64 `json:"pct"` // 涨跌幅，0.25 表示涨 25%
}

// HistoryExport 是 history.json 的内容。
type HistoryExport struct {
	Days    []string        `json:"days"`
	Series  []HistorySeries `json:"series"`
	Changes []PriceChange   `json:"changes"`
}

// BuildHistory 从库里汇总历史序列与变价事件。
func BuildHistory(store *Store) (*HistoryExport, error) {
	out := &HistoryExport{Days: []string{}, Series: []HistorySeries{}, Changes: []PriceChange{}}

	days, err := store.distinctDays()
	if err != nil {
		return nil, err
	}
	out.Days = days

	series, err := store.minPriceSeries()
	if err != nil {
		return nil, err
	}
	out.Series = series

	// 只有一天数据时不可能有变价，跳过那条比较贵的窗口查询
	if len(days) >= 2 {
		changes, err := store.priceChanges(200)
		if err != nil {
			return nil, err
		}
		out.Changes = changes
	}
	return out, nil
}

func (s *Store) distinctDays() ([]string, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT date FROM price_snapshots WHERE parse_status='ok' ORDER BY date`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		if len(d) >= 10 {
			d = d[:10]
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// 每个 (产品, 档位, 周期, 日期) 取当天美元价最低的那条记录。
func (s *Store) minPriceSeries() ([]HistorySeries, error) {
	rows, err := s.db.Query(`
		SELECT date, product_id, tier, period, storefront, usd_micro
		FROM (
			SELECT date, product_id, tier, period, storefront, usd_micro,
			       ROW_NUMBER() OVER (
			           PARTITION BY date, product_id, tier, period
			           ORDER BY usd_micro ASC, storefront ASC
			       ) AS rn
			FROM price_snapshots
			WHERE parse_status = 'ok' AND usd_micro IS NOT NULL
		) t
		WHERE rn = 1
		ORDER BY product_id, tier, period, date`)
	if err != nil {
		return nil, fmt.Errorf("查历史最低价失败: %w", err)
	}
	defer rows.Close()

	byKey := map[string]*HistorySeries{}
	var order []string
	for rows.Next() {
		var date, productID, tier, period, storefront string
		var usdMicro int64
		if err := rows.Scan(&date, &productID, &tier, &period, &storefront, &usdMicro); err != nil {
			return nil, err
		}
		if len(date) >= 10 {
			date = date[:10]
		}
		key := productID + "|" + tier + "|" + period
		sr, ok := byKey[key]
		if !ok {
			sr = &HistorySeries{ProductID: productID, Tier: tier, Period: period}
			if r := findRule(productID, tier, period); r != nil {
				sr.TierEn, sr.TierZh = r.TierEn, r.TierZh
			}
			byKey[key] = sr
			order = append(order, key)
		}
		sr.Points = append(sr.Points, HistoryPoint{
			Date: date, USD: float64(usdMicro) / 1e6, Storefront: storefront,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]HistorySeries, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	return out, nil
}

// 用窗口函数比较同一 (产品, 档位, 周期, 地区) 相邻两天的本地价格。
func (s *Store) priceChanges(limit int) ([]PriceChange, error) {
	rows, err := s.db.Query(`
		SELECT date, prev_date, product_id, tier, period, storefront, currency,
		       prev_raw, raw_price, prev_minor, amount_minor
		FROM (
			SELECT date, product_id, tier, period, storefront, currency,
			       raw_price, amount_minor,
			       LAG(amount_minor) OVER w AS prev_minor,
			       LAG(raw_price)    OVER w AS prev_raw,
			       LAG(date)         OVER w AS prev_date
			FROM price_snapshots
			WHERE parse_status = 'ok' AND amount_minor IS NOT NULL
			WINDOW w AS (
				PARTITION BY product_id, tier, period, storefront
				ORDER BY date
			)
		) t
		WHERE prev_minor IS NOT NULL
		  AND prev_minor <> amount_minor
		ORDER BY date DESC, ABS(amount_minor - prev_minor) DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("查变价事件失败: %w", err)
	}
	defer rows.Close()

	var out []PriceChange
	for rows.Next() {
		var c PriceChange
		var prevMinor, curMinor int64
		if err := rows.Scan(&c.Date, &c.PrevDate, &c.ProductID, &c.Tier, &c.Period,
			&c.Storefront, &c.Currency, &c.FromRaw, &c.ToRaw, &prevMinor, &curMinor); err != nil {
			return nil, err
		}
		if len(c.Date) >= 10 {
			c.Date = c.Date[:10]
		}
		if len(c.PrevDate) >= 10 {
			c.PrevDate = c.PrevDate[:10]
		}
		if r := findRule(c.ProductID, c.Tier, c.Period); r != nil {
			c.TierEn, c.TierZh = r.TierEn, r.TierZh
		}
		if prevMinor != 0 {
			c.Pct = float64(curMinor-prevMinor) / float64(prevMinor)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	return out, nil
}
