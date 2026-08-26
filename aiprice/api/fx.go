// 汇率获取。
//
// 主源 open.er-api.com（166 种货币），备源 jsDelivr 上的 currency-api（341 种）。
// 两个都免费、不需要 API Key，实测当天 USD/CNY 分别 6.737 / 6.722（差 0.2%，属正常）。
//
// 不要用 ECB / frankfurter.app：只覆盖约 30 种主要货币，NGN / COP / EGP / PKR / TZS
// 这些价格差异最大的地区全都没有。竞品 geopriced.com 用的就是 ECB，
// 大概率就是它只做 46 个国家的原因（调研报告 §3.5）。
//
// 汇率必须**逐日落库存档**：历史价格要配当时的汇率才有意义，
// 不能用今天的汇率去换算上个月的价格。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FXRates 是某一天以 USD 为基准的汇率表。
type FXRates struct {
	Date   string             // YYYY-MM-DD
	Base   string             // 恒为 USD
	Source string             // 实际取数的源
	Rates  map[string]float64 // 1 USD = N 单位目标币
}

// Rate 返回 1 USD 折合多少目标币。
func (f *FXRates) Rate(currency string) (float64, bool) {
	r, ok := f.Rates[strings.ToUpper(currency)]
	return r, ok
}

// ToUSD 把某币种的金额折算成美元。
func (f *FXRates) ToUSD(amount float64, currency string) (float64, bool) {
	r, ok := f.Rate(currency)
	if !ok || r == 0 {
		return 0, false
	}
	return amount / r, true
}

// FetchFX 依次尝试主源和备源，任一成功即返回。
func FetchFX(client *http.Client) (*FXRates, error) {
	var errs []string
	for _, src := range []struct {
		name string
		fn   func(*http.Client) (*FXRates, error)
	}{
		{"open.er-api.com", fetchErAPI},
		{"jsdelivr/currency-api", fetchCurrencyAPI},
	} {
		fx, err := src.fn(client)
		if err == nil && len(fx.Rates) > 0 {
			fx.Source = src.name
			return fx, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", src.name, err))
	}
	return nil, fmt.Errorf("所有汇率源都失败了 [%s]", strings.Join(errs, "; "))
}

func fetchErAPI(client *http.Client) (*FXRates, error) {
	var body struct {
		Result         string             `json:"result"`
		TimeLastUpdate int64              `json:"time_last_update_unix"`
		Rates          map[string]float64 `json:"rates"`
	}
	if err := getJSON(client, "https://open.er-api.com/v6/latest/USD", &body); err != nil {
		return nil, err
	}
	if body.Result != "success" {
		return nil, fmt.Errorf("接口返回 result=%q", body.Result)
	}
	date := time.Unix(body.TimeLastUpdate, 0).UTC().Format("2006-01-02")
	if body.TimeLastUpdate == 0 {
		date = time.Now().UTC().Format("2006-01-02")
	}
	return &FXRates{Date: date, Base: "USD", Rates: upperKeys(body.Rates)}, nil
}

func fetchCurrencyAPI(client *http.Client) (*FXRates, error) {
	var body struct {
		Date string             `json:"date"`
		USD  map[string]float64 `json:"usd"`
	}
	const url = "https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api@latest/v1/currencies/usd.json"
	if err := getJSON(client, url, &body); err != nil {
		return nil, err
	}
	if body.Date == "" {
		body.Date = time.Now().UTC().Format("2006-01-02")
	}
	return &FXRates{Date: body.Date, Base: "USD", Rates: upperKeys(body.USD)}, nil
}

func upperKeys(in map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(in))
	for k, v := range in {
		out[strings.ToUpper(k)] = v
	}
	return out
}

func getJSON(client *http.Client, url string, dst any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "aiprice/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}
