// 价格字符串解析。
//
// App Store 返回的是**已按地区本地化格式化**的价格字符串，同一个 SKU 在 158 个地区
// 实测有 78 种不同格式。踩过的坑（详见 ../../AI订阅全球价格站可行性调研.md §5.1）：
//
//	$19.99        美国      点是小数点
//	$ 99.900,00   哥伦比亚  同样是 $，但点是千分位、逗号是小数点
//	$999.999      智利      点是千分位，整个数没有小数
//	¥3,000        日本      逗号是千分位，零小数币种
//	9 999,99 Kč   捷克      NBSP 当千分位
//	HUF8,990.00   匈牙利    看着像零小数币种，其实是两位小数
//	Rp 349ribu    印尼      ribu = 千
//	Rp 1,999juta  印尼      juta = 百万，且这里的逗号是小数点（= 1,999,000）
//
// 两条铁律：
//  1. 绝不从货币符号推断币种——`$` 可能是 USD/COP/CLP/MXN。币种必须由调用方从
//     「地区 → ISO 币种」映射传入。
//  2. 小数位数来自 ISO 4217 exponent 表，不能凭直觉。HUF/COP/TZS 都是两位小数。
package main

import (
	"fmt"
	"math/big"
	"strings"
)

// ISO 4217 中小数位不为 2 的币种。其余一律按 2 位处理。
var (
	exponent0 = map[string]bool{
		"BIF": true, "CLP": true, "DJF": true, "GNF": true, "ISK": true,
		"JPY": true, "KMF": true, "KRW": true, "PYG": true, "RWF": true,
		"UGX": true, "UYI": true, "VND": true, "VUV": true, "XAF": true,
		"XOF": true, "XPF": true,
	}
	exponent3 = map[string]bool{
		"BHD": true, "IQD": true, "JOD": true, "KWD": true,
		"LYD": true, "OMR": true, "TND": true,
	}
)

// CurrencyExponent 返回该币种的小数位数。
func CurrencyExponent(currency string) int {
	switch c := strings.ToUpper(currency); {
	case exponent0[c]:
		return 0
	case exponent3[c]:
		return 3
	default:
		return 2
	}
}

// 本地化数量词。出现时价格串里的分隔符一律按小数点处理：
// "Rp 1,999juta" 是 1.999 百万 = 1,999,000，不是 1999 百万。
var magnitudes = []struct {
	word  string
	pow10 int
}{
	{"juta", 6}, // 印尼语：百万
	{"ribu", 3}, // 印尼语：千
	{"jt", 6},
	{"rb", 3},
}

// 各种不间断空格，Apple 大量使用 U+00A0。
var spaceReplacer = strings.NewReplacer(
	" ", " ", // NO-BREAK SPACE
	" ", " ", // NARROW NO-BREAK SPACE
	" ", " ", // FIGURE SPACE
	" ", " ", // THIN SPACE
)

// ParsePrice 把 App Store 的格式化价格串解析成该币种的最小货币单位（分/厘）。
// currency 必须是调用方已知的 ISO 币种，不从字符串里猜。
func ParsePrice(raw, currency string) (int64, error) {
	s := spaceReplacer.Replace(raw)

	// 1) 数量词：取出倍数并从串里移除
	mag := 0
	lower := strings.ToLower(s)
	for _, m := range magnitudes {
		if i := strings.Index(lower, m.word); i >= 0 {
			mag = m.pow10
			s = s[:i] + s[i+len(m.word):]
			break
		}
	}

	// 2) 只保留数字和分隔符
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' || r == ',' || r == ' ' {
			b.WriteRune(r)
		}
	}
	t := strings.TrimSpace(b.String())

	// 3) 数字之间的空格是千分位（捷克 "9 999,99"）
	t = removeInnerSpaces(t)
	if !strings.ContainsAny(t, "0123456789") {
		return 0, fmt.Errorf("价格串里没有数字: %q", raw)
	}

	exp := CurrencyExponent(currency)

	// 4) 判定最后一个分隔符是不是小数点
	intPart, fracPart := t, ""
	if last := strings.LastIndexAny(t, ".,"); last >= 0 {
		sep := t[last]
		frac := t[last+1:]
		isDecimal := false
		switch {
		case mag > 0:
			// 有数量词时分隔符必然是小数点
			isDecimal = true
		case exp > 0 && len(frac) == exp && strings.Count(t, string(sep)) == 1:
			isDecimal = true
		}
		if isDecimal {
			intPart, fracPart = t[:last], frac
		}
	}
	intPart = strings.NewReplacer(".", "", ",", "").Replace(intPart)
	if intPart == "" {
		intPart = "0"
	}
	if strings.ContainsAny(fracPart, ".,") {
		return 0, fmt.Errorf("小数部分含分隔符，格式无法识别: %q", raw)
	}

	// 5) 精确换算成最小货币单位：value * 10^mag * 10^exp
	num := new(big.Rat)
	if _, ok := num.SetString(intPart + "." + pad(fracPart)); !ok {
		return 0, fmt.Errorf("数字解析失败: %q (清洗后 %q)", raw, t)
	}
	num.Mul(num, ratPow10(mag+exp))
	if !num.IsInt() {
		return 0, fmt.Errorf("换算成最小货币单位后不是整数: %q (%s)", raw, currency)
	}
	if !num.Num().IsInt64() {
		return 0, fmt.Errorf("价格超出 int64 范围: %q", raw)
	}
	return num.Num().Int64(), nil
}

func pad(frac string) string {
	if frac == "" {
		return "0"
	}
	return frac
}

func removeInnerSpaces(t string) string {
	var b strings.Builder
	for i := 0; i < len(t); i++ {
		if t[i] == ' ' {
			continue
		}
		b.WriteByte(t[i])
	}
	return b.String()
}

func ratPow10(n int) *big.Rat {
	r := new(big.Rat).SetInt64(1)
	ten := new(big.Rat).SetInt64(10)
	for i := 0; i < n; i++ {
		r.Mul(r, ten)
	}
	return r
}

// FormatMinor 把最小货币单位还原成人类可读的小数字符串（不带货币符号）。
func FormatMinor(minor int64, currency string) string {
	exp := CurrencyExponent(currency)
	if exp == 0 {
		return fmt.Sprintf("%d", minor)
	}
	div := int64(1)
	for i := 0; i < exp; i++ {
		div *= 10
	}
	neg := ""
	if minor < 0 {
		neg, minor = "-", -minor
	}
	return fmt.Sprintf("%s%d.%0*d", neg, minor/div, exp, minor%div)
}
