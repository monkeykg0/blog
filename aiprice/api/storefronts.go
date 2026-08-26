// App Store 地区表：地区码 → ISO 币种 + 中英文名 + 旗帜。
//
// 币种映射是价格解析的基础事实（见 price.go 的说明），不能靠符号猜。
// data/storefronts.tsv 由 iTunes Lookup API 实测 + Node 的 CLDR 数据生成，
// 变化极慢，用 `aiprice refresh-storefronts` 每月刷新一次即可。
package main

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed data/storefronts.tsv
var storefrontsTSV string

// Storefront 是一个 App Store 地区。
type Storefront struct {
	Code       string `json:"code"`     // 两位小写地区码，如 "us"
	Currency   string `json:"currency"` // ISO 4217 币种
	Flag       string `json:"flag"`     // 旗帜 emoji
	NameEn     string `json:"nameEn"`
	NameZh     string `json:"nameZh"`
	CurrencyEn string `json:"currencyEn"`
	CurrencyZh string `json:"currencyZh"`
}

var (
	storefronts   []Storefront
	storefrontMap map[string]Storefront
)

func init() {
	lines := strings.Split(strings.TrimSpace(storefrontsTSV), "\n")
	storefrontMap = make(map[string]Storefront, len(lines))
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue // 表头
		}
		f := strings.Split(line, "\t")
		if len(f) < 7 {
			panic(fmt.Sprintf("storefronts.tsv 第 %d 行字段不足: %q", i+1, line))
		}
		s := Storefront{
			Code: f[0], Currency: f[1], Flag: f[2],
			NameEn: f[3], NameZh: f[4], CurrencyEn: f[5], CurrencyZh: f[6],
		}
		storefronts = append(storefronts, s)
		storefrontMap[s.Code] = s
	}
	if len(storefronts) == 0 {
		panic("storefronts.tsv 为空")
	}
}

// Storefronts 返回全部地区（按地区码排序）。
func Storefronts() []Storefront { return storefronts }

// LookupStorefront 按地区码取地区信息。
func LookupStorefront(code string) (Storefront, bool) {
	s, ok := storefrontMap[strings.ToLower(code)]
	return s, ok
}
