package main

import "testing"

// 用例全部来自 2026-08-26 对 158 个 App Store 地区的真实抓取结果。
// 新增地区或 Apple 改格式时，把新的原始串加到这里再改解析器。
func TestParsePrice(t *testing.T) {
	cases := []struct {
		raw      string
		currency string
		want     int64 // 最小货币单位
	}{
		// 常规两位小数
		{"$19.99", "USD", 1999},
		{"USD 19.99", "USD", 1999},
		{"22,99 USD", "USD", 2299},
		{"19,99 $US", "USD", 1999}, // 法语区的美元写法
		{"€22.99", "EUR", 2299},
		{"22,99 €", "EUR", 2299},
		{"£19.99", "GBP", 1999},
		{"R$ 99,90", "BRL", 9990},
		{"₺999,99", "TRY", 99999},
		{"₺9.999,99", "TRY", 999999},
		{"CHF 21.99", "CHF", 2199},
		{"AED 79.99", "AED", 7999},
		{"QAR 69.99", "QAR", 6999},
		{"EGP 999.99", "EGP", 99999},
		{"Rs 4,900.00", "PKR", 490000},
		{"₱ 999.00", "PHP", 99900},
		{"₹ 1,999", "INR", 199900}, // 印度不显示小数
		{"99,99 zł", "PLN", 9999},
		{"179,00 kr", "DKK", 17900},
		{"₪69.90", "ILS", 6990},

		// 同符号、相反约定 —— 最容易出事的一组
		{"$ 99.900,00", "COP", 9990000}, // 哥伦比亚：点千分位、逗号小数点
		{"$999.999", "CLP", 999999},          // 智利：零小数币种，点是千分位
		{"$24.99", "CAD", 2499},

		// 看起来像零小数、其实是两位小数（曾经算大 100 倍）
		{"HUF8,990.00", "HUF", 899000},
		{"49,900.00 TZS", "TZS", 4990000},

		// 真正的零小数币种
		{"¥3,000", "JPY", 3000},
		{"￦99,000", "KRW", 99000},
		{"499.000đ", "VND", 499000},

		// NBSP 当千分位
		{"9 999,99 Kč", "CZK", 999999},

		// 印尼数量词
		{"Rp 349ribu", "IDR", 34900000},   // 349,000 IDR
		{"Rp 1,999juta", "IDR", 199900000}, // 1,999,000 IDR —— 逗号在这里是小数点
		{"Rp 3,499juta", "IDR", 349900000},
		{"Rp 75ribu", "IDR", 7500000},

		// 三位小数币种（当前地区表里没有，防御性覆盖）
		{"BHD 7.999", "BHD", 7999},
	}

	for _, c := range cases {
		got, err := ParsePrice(c.raw, c.currency)
		if err != nil {
			t.Errorf("ParsePrice(%q, %s) 报错: %v", c.raw, c.currency, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParsePrice(%q, %s) = %d, 期望 %d (%s vs %s)",
				c.raw, c.currency, got, c.want,
				FormatMinor(got, c.currency), FormatMinor(c.want, c.currency))
		}
	}
}

func TestParsePriceErrors(t *testing.T) {
	for _, raw := range []string{"", "Free", "无料", "--"} {
		if v, err := ParsePrice(raw, "USD"); err == nil {
			t.Errorf("ParsePrice(%q) 本应报错，却返回 %d", raw, v)
		}
	}
}

func TestCurrencyExponent(t *testing.T) {
	cases := map[string]int{
		"USD": 2, "EUR": 2, "HUF": 2, "COP": 2, "TZS": 2, "IDR": 2,
		"JPY": 0, "KRW": 0, "VND": 0, "CLP": 0, "ISK": 0,
		"BHD": 3, "KWD": 3, "OMR": 3,
	}
	for cur, want := range cases {
		if got := CurrencyExponent(cur); got != want {
			t.Errorf("CurrencyExponent(%s) = %d, 期望 %d", cur, got, want)
		}
	}
}

func TestFormatMinor(t *testing.T) {
	cases := []struct {
		minor    int64
		currency string
		want     string
	}{
		{1999, "USD", "19.99"},
		{3000, "JPY", "3000"},
		{9990000, "COP", "99900.00"},
		{34900000, "IDR", "349000.00"},
		{7999, "BHD", "7.999"},
		{5, "USD", "0.05"},
	}
	for _, c := range cases {
		if got := FormatMinor(c.minor, c.currency); got != c.want {
			t.Errorf("FormatMinor(%d, %s) = %s, 期望 %s", c.minor, c.currency, got, c.want)
		}
	}
}
