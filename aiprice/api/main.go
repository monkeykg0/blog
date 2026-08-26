// aiprice：AI 订阅全球价格抓取与导出。
//
//	aiprice crawl              抓一次全量并落库（由 systemd timer 每日触发）
//	aiprice crawl -dry-run     只抓不落库，用来验证解析规则
//	aiprice export -out DIR    从库里导出静态站构建所需的 JSON
//	aiprice check              自检：地区表、SKU 规则、汇率源、出口 IP
//
// 背景与实测数据见 ../../AI订阅全球价格站可行性调研.md
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	log.SetFlags(log.Ltime)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "crawl":
		cmdCrawl(os.Args[2:])
	case "export":
		cmdExport(os.Args[2:])
	case "check":
		cmdCheck(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `aiprice — AI 订阅全球价格抓取

用法:
  aiprice crawl [-dry-run]     抓取全部产品 × 全部地区并落库
  aiprice export -out DIR      导出静态站构建用的 JSON
  aiprice check                自检环境（出口 IP / 汇率源 / 目录数据）

环境变量:
  MYSQL_DSN   MySQL 连接串，默认 aiprice:aiprice@tcp(127.0.0.1:3306)/aiprice?parseTime=true
`)
}

func mustStore() *Store {
	dsn := env("MYSQL_DSN", "aiprice:aiprice@tcp(127.0.0.1:3306)/aiprice?parseTime=true")
	store, err := OpenStore(dsn)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	return store
}

func cmdCrawl(args []string) {
	fs := flag.NewFlagSet("crawl", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "只抓取和解析，不写数据库")
	limit := fs.Int("limit", 0, "只抓前 N 个地区（调试用，0 表示全部）")
	product := fs.String("product", "", "只抓某个产品（调试用，如 chatgpt）")
	_ = fs.Parse(args)

	var store *Store
	if !*dryRun {
		store = mustStore()
		defer store.Close()
	}

	res, err := Crawl(store, CrawlOptions{DryRun: *dryRun, Limit: *limit, Product: *product})
	if res != nil {
		printCrawlSummary(res)
	}
	if err != nil {
		log.Fatalf("抓取失败: %v", err)
	}
}

func printCrawlSummary(res *CrawlResult) {
	log.Printf("──────── 抓取汇总 %s ────────", res.Date)
	log.Printf("成功 %d  未上架 %d  失败 %d  耗时 %s",
		res.OK, res.Absent, res.Errors, res.Duration.Round(time.Second))

	if len(res.UnknownSKU) > 0 {
		log.Printf("⚠️  出现了 %d 种没见过的 SKU，需要到 catalog.go 里补规则:", len(res.UnknownSKU))
		for _, k := range res.UnknownSKU {
			log.Printf("      %s", k)
		}
	}
	if len(res.Unclassified) > 0 {
		log.Printf("⚠️  %d 条价格落在已知区间之外（厂商调价？解析出错？必须人工确认）:", len(res.Unclassified))
		for _, k := range res.Unclassified {
			log.Printf("      %s", k)
		}
	}
	if len(res.UnknownSKU) == 0 && len(res.Unclassified) == 0 {
		log.Printf("✅ 所有 SKU 都命中白名单规则")
	}
}

func cmdExport(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	out := fs.String("out", "../web/src/data", "JSON 输出目录")
	date := fs.String("date", "", "导出哪一天的数据，留空表示最新一天")
	_ = fs.Parse(args)

	store := mustStore()
	defer store.Close()

	if err := Export(store, *out, *date); err != nil {
		log.Fatalf("导出失败: %v", err)
	}
}

func cmdCheck(args []string) {
	_ = args
	fmt.Printf("地区表     : %d 个地区\n", len(Storefronts()))
	fmt.Printf("产品       : %d 个\n", len(Products))
	fmt.Printf("SKU 规则   : %d 条\n", len(SKURules))

	client := &http.Client{Timeout: 20 * time.Second}

	fx, err := FetchFX(client)
	if err != nil {
		fmt.Printf("汇率源     : ❌ %v\n", err)
	} else {
		fmt.Printf("汇率源     : ✅ %s  %s  %d 种货币\n", fx.Source, fx.Date, len(fx.Rates))
		missing := missingCurrencies(fx)
		if len(missing) > 0 {
			fmt.Printf("           ⚠️  地区表用到但汇率源没有的币种: %s\n", strings.Join(missing, ", "))
		}
	}

	// 出口 IP 检查：中国大陆 IP 会被 Apple 强制锁到 /cn/，抓出来全是错数据
	f := NewFetcher()
	f.Throttle = 0
	if _, err := f.Offers("us", Products[0].AppStoreID); err != nil {
		fmt.Printf("App Store  : ❌ %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("App Store  : ✅ 美区商店页可正常访问（出口 IP 不在中国大陆）\n")
}

func missingCurrencies(fx *FXRates) []string {
	seen := map[string]bool{}
	var missing []string
	for _, sf := range Storefronts() {
		if seen[sf.Currency] {
			continue
		}
		seen[sf.Currency] = true
		if _, ok := fx.Rate(sf.Currency); !ok {
			missing = append(missing, sf.Currency)
		}
	}
	return missing
}

// 供 store.go 使用的小工具，避免为此引入额外依赖
func splitOnSemicolon(s string) []string { return strings.Split(s, ";") }
func trimSpace(s string) string          { return strings.TrimSpace(s) }
func contains(s, sub string) bool        { return strings.Contains(s, sub) }
