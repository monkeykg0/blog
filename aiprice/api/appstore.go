// App Store 抓取。
//
// 数据源是商店页 HTML 里的 <script id="serialized-server-data"> JSON，公开免鉴权。
// 里面有个 title 为「App 内购买」的 Annotation 节点，items_V3 是一组 textPair：
//
//	{"$kind":"textPair","leadingText":"ChatGPT Plus","trailingText":"$19.99"}
//
// 两个必须知道的限制（调研报告 §5.2 / §5.5）：
//   - Apple 最多只展示 **10 条**内购，按该地区热度排序。所以某档位查不到 ≠ 该地区没有，
//     只是没挤进前 10。ChatGPT/Claude 主力档 158 地区无损，Gemini Ultra 只有 87 个地区。
//   - **中国大陆 IP 抓不了**：无论指定哪个地区码，一律 302 到 /cn/iphone/today。
//     爬虫必须跑在海外机器上，本地开发要走代理。
//
// 合规：apps.apple.com/robots.txt 只禁 /api/*、/v1/*、/WebObjects/*、*/search?*，
// 我们抓的 /{地区}/app/id{ID} 不在禁止列表内，且被官方 sitemap 收录。
// 不要改用 amp-api.apps.apple.com（落在 Disallow: /v1/* 里，且需要绕鉴权）。
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"

// ErrNotAvailable 表示该产品未在该地区上架（Apple 返回 404）。
// 这是正常情况，不是抓取失败——四个产品的上架地区本来就不同。
var ErrNotAvailable = errors.New("该地区未上架")

var serverDataRe = regexp.MustCompile(
	`(?s)<script[^>]*id="serialized-server-data"[^>]*>(.*?)</script>`)

// RawOffer 是一条未经归一化的内购项目。
type RawOffer struct {
	Name  string // Apple 展示名
	Price string // Apple 已本地化格式化的价格串
}

// Fetcher 抓取 App Store 商店页。
type Fetcher struct {
	Client *http.Client
	// Throttle 是两次请求之间的最小间隔。实测不加间隔第 5 个请求就吃 429；
	// 1.2s 下跑完 632 次请求零限流。别调小。
	Throttle time.Duration
	// MaxRetries 是遇到 429/5xx 时的重试次数，采用指数退避。
	MaxRetries int

	last time.Time
}

// NewFetcher 返回一个按实测参数配置好的抓取器。
func NewFetcher() *Fetcher {
	return &Fetcher{
		Client: &http.Client{Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Apple 对中国大陆 IP 会把所有地区码重定向到 /cn/iphone/today。
				// 与其静默拿到错数据，不如在这里就报错。
				if strings.Contains(req.URL.Path, "/iphone/today") ||
					strings.Contains(req.URL.Path, "/ipad/today") {
					return fmt.Errorf("被重定向到 Today 页——当前出口 IP 很可能在中国大陆，"+
						"Apple 强制锁 /cn/ 商店，请在海外机器上运行或配置代理 (%s)", req.URL)
				}
				if len(via) >= 10 {
					return errors.New("重定向次数过多")
				}
				return nil
			}},
		Throttle:   1200 * time.Millisecond,
		MaxRetries: 3,
	}
}

// Offers 抓取某地区某 App 的内购列表。
func (f *Fetcher) Offers(storefront string, appID int64) ([]RawOffer, error) {
	url := fmt.Sprintf("https://apps.apple.com/%s/app/id%d", storefront, appID)

	var lastErr error
	for attempt := 0; attempt <= f.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<attempt) * 3 * time.Second) // 指数退避
		}
		f.wait()

		body, status, err := f.get(url)
		switch {
		case err != nil:
			lastErr = err
			// 重定向到 Today 页说明出口 IP 不对，重试无意义
			if strings.Contains(err.Error(), "Today 页") {
				return nil, err
			}
			continue
		case status == http.StatusNotFound:
			return nil, ErrNotAvailable
		case status == http.StatusTooManyRequests || status >= 500:
			lastErr = fmt.Errorf("HTTP %d", status)
			continue
		case status != http.StatusOK:
			return nil, fmt.Errorf("HTTP %d", status)
		}

		offers, err := parseOffers(body)
		if err != nil {
			return nil, err
		}
		return offers, nil
	}
	return nil, fmt.Errorf("重试 %d 次后仍失败: %w", f.MaxRetries, lastErr)
}

func (f *Fetcher) wait() {
	if d := f.Throttle - time.Since(f.last); d > 0 {
		time.Sleep(d)
	}
	f.last = time.Now()
}

func (f *Fetcher) get(url string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	// 商店页约 0.5–1.5MB，留足余量并封顶防止异常响应打爆内存
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// serialized-server-data 的结构很深且不稳定，与其硬绑 schema，
// 不如递归找到那个 textPairs 数组——它的形状是稳定的。
func parseOffers(html []byte) ([]RawOffer, error) {
	m := serverDataRe.FindSubmatch(html)
	if m == nil {
		return nil, errors.New("页面里没有 serialized-server-data，Apple 可能改版了")
	}
	var doc any
	if err := json.Unmarshal(m[1], &doc); err != nil {
		return nil, fmt.Errorf("serialized-server-data 不是合法 JSON: %w", err)
	}
	var out []RawOffer
	findTextPairs(doc, &out)
	return out, nil
}

func findTextPairs(node any, out *[]RawOffer) {
	switch v := node.(type) {
	case map[string]any:
		// 形式一：{"textPairs": [["名称","价格"], ...]}
		if pairs, ok := v["textPairs"].([]any); ok && len(*out) == 0 {
			for _, p := range pairs {
				if pair, ok := p.([]any); ok && len(pair) >= 2 {
					name, ok1 := pair[0].(string)
					price, ok2 := pair[1].(string)
					if ok1 && ok2 {
						*out = append(*out, RawOffer{Name: name, Price: price})
					}
				}
			}
			if len(*out) > 0 {
				return
			}
		}
		// 形式二：{"$kind":"textPair","leadingText":..,"trailingText":..}
		if v["$kind"] == "textPair" {
			name, ok1 := v["leadingText"].(string)
			price, ok2 := v["trailingText"].(string)
			if ok1 && ok2 {
				*out = append(*out, RawOffer{Name: name, Price: price})
				return
			}
		}
		for _, child := range v {
			findTextPairs(child, out)
		}
	case []any:
		for _, child := range v {
			findTextPairs(child, out)
		}
	}
}
