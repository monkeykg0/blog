// 中英双语文案。
//
// 地区名和货币名不在这里——那两套由 CLDR 数据生成，已经随 storefronts.json 一起导出
// （每条记录同时带 nameEn / nameZh），不需要手工维护 158 × 2 条翻译。
// 这里只放页面文案。

export const LANGS = ["en", "zh"] as const;
export type Lang = (typeof LANGS)[number];
export const DEFAULT_LANG: Lang = "en";

/** <html lang> 与 hreflang 用的标准语言标签 */
export const HTML_LANG: Record<Lang, string> = {
	en: "en",
	zh: "zh-Hans",
};

const STRINGS = {
	siteName: { en: "AIPRICE", zh: "AIPRICE" },
	tagline: {
		en: "AI subscription prices, worldwide",
		zh: "AI 订阅全球价格观测",
	},

	navProducts: { en: "Products", zh: "产品" },
	navRegions: { en: "Regions", zh: "地区" },
	navHistory: { en: "History", zh: "历史" },
	navMethod: { en: "Method", zh: "方法论" },

	historyTitle: { en: "Lowest price worldwide, by day", zh: "各产品全球最低价，逐日" },
	historySub: {
		en: "Each line is that product's cheapest storefront price on that day, in USD. A line falling can mean a price cut or simply a weaker local currency.",
		zh: "每条线是该产品当天在全部地区中的最低折算美元价。线的下行既可能来自降价，也可能来自当地货币贬值。",
	},
	historyEmpty: {
		en: "Only one day of data so far",
		zh: "目前只有一天数据",
	},
	historyEmptyBody: {
		en: "Trend lines need at least two daily snapshots. Collection started {date} and runs once a day — the chart fills in from tomorrow. The figures below are today's starting values.",
		zh: "画趋势线至少需要两天快照。数据从 {date} 开始采集，每日一次，曲线将从明天起逐日生长。下面是今天的起点值。",
	},
	changesTitle: { en: "Price changes", zh: "变价事件" },
	changesEmpty: {
		en: "No price changes recorded yet — that needs at least two snapshots to compare.",
		zh: "尚未记录到变价 —— 这需要至少两次快照才能比较。",
	},
	changesNote: {
		en: "Compared against the previous snapshot, storefront by storefront. Only the local currency price is compared: an exchange-rate move is not a price change.",
		zh: "与上一次抓取逐地区比对。只比较本地货币标价——汇率波动不算变价。",
	},
	whyHistory: { en: "Why keep history", zh: "为什么记录历史" },
	daysCollected: { en: "{n} days collected", zh: "已积累 {n} 天" },

	homeEyebrow: {
		en: "Measured daily · {n} App Store storefronts",
		zh: "每日实测 · {n} 个 App Store 地区",
	},
	homeHeadline: {
		en: "The priciest storefront charges half again what the cheapest does",
		zh: "同一个订阅，最贵的地区比最便宜的地区多付一半",
	},
	homeStandfirst: {
		en: "Every day this site reads all {n} Apple App Store storefronts and records the raw price string Apple returns for ChatGPT, Claude, Gemini and Grok, converted at that day's mid-market rate. No recommendations, no gift cards — just the prices.",
		zh: "本站每日自动抓取 Apple App Store 全部 {n} 个地区商店页，记录 ChatGPT、Claude、Gemini、Grok 的订阅价格原始字符串与抓取时间，按当日中间价折算美元与人民币。不做推荐，不卖礼品卡，只呈现价格事实。",
	},

	thisRun: { en: "This run", zh: "本次抓取" },
	dataDate: { en: "Data date", zh: "数据日期" },
	coverage: { en: "Storefronts", zh: "覆盖地区" },
	priceRecords: { en: "Price records", zh: "价格记录" },
	fxRate: { en: "FX rate", zh: "汇率" },

	featuredTiers: { en: "Headline tiers · global range", zh: "主力档位 · 全球价格区间" },
	tableNote: {
		en: "Monthly in-app purchase prices from each storefront, converted to USD.",
		zh: "价格为各地区 App Store 内购月付价，已折算美元。",
	},

	colProduct: { en: "Product / tier", zh: "产品 / 档位" },
	colSpread: { en: "Global spread", zh: "全球分布" },
	colLowest: { en: "Lowest", zh: "全球最低" },
	colUS: { en: "United States", zh: "美国" },
	colGap: { en: "Max spread", zh: "最大价差" },
	colRank: { en: "Rank", zh: "排名" },
	colRegion: { en: "Region", zh: "地区" },
	colLocal: { en: "Local price", zh: "本地价格" },
	colVsLowest: { en: "vs lowest", zh: "较最低" },
	colTier: { en: "Tier", zh: "档位" },
	colPeriod: { en: "Billing", zh: "周期" },
	colGlobalRank: { en: "Global rank", zh: "全球排名" },

	monthly: { en: "Monthly", zh: "月付" },
	annual: { en: "Annual", zh: "年付" },

	ofN: { en: "of {n}", zh: "/ {n}" },
	regionsWord: { en: "{n} storefronts", zh: "{n} 个地区" },
	median: { en: "Median", zh: "中位数" },
	highest: { en: "Highest", zh: "最高" },
	lowest: { en: "Lowest", zh: "最低" },

	rankingTitle: { en: "{tier} — global price ranking", zh: "{tier} 全球价格排行" },
	rankingSub: {
		en: "{n} App Store storefronts · sorted by converted USD price · data {date}",
		zh: "{n} 个 App Store 地区 · 按折算美元价升序 · 数据日期 {date}",
	},
	convertTo: { en: "Convert to", zh: "折算" },

	regionTitle: { en: "{name} App Store", zh: "{name} App Store" },
	regionSub: {
		en: "Billed in {cur} · {n} subscription tiers on sale across four products",
		zh: "结算币种 {cur} · 四个产品共 {n} 个订阅档位在售",
	},
	regionsIndexTitle: { en: "All storefronts", zh: "全部地区" },
	regionsIndexSub: {
		en: "{n} App Store storefronts, sorted by the cheapest ChatGPT Plus price.",
		zh: "{n} 个 App Store 地区，按 ChatGPT Plus 月付价从低到高排列。",
	},
	notListed: {
		en: "Some tiers are missing here. Apple shows at most 10 in-app purchases per app, ranked by local popularity — a tier that does not appear is not necessarily unavailable, it just did not make the top 10.",
		zh: "本地区部分档位未返回。Apple 每个 App 最多展示 10 条内购项目，按该地区热度排序——未返回不代表该地区不售，只是没进前 10。",
	},

	methodTitle: { en: "How these numbers are produced", zh: "这些数字是怎么来的" },

	notAffiliated: {
		en: "Not affiliated with OpenAI, Anthropic, Google or xAI.",
		zh: "本站与 OpenAI、Anthropic、Google、xAI 均无关联。",
	},
	disclaimer: {
		en: "Prices are read from Apple's public App Store pages; FX from {fx}. For reference only — the amount you are actually charged is set by the platform. Switching App Store region requires a valid payment method in that region and may breach the Apple Media Services Terms.",
		zh: "价格数据取自 Apple App Store 各地区公开商店页，汇率取自 {fx}，仅供参考，以各平台实际结算为准。切换 App Store 地区需要该地区的有效支付方式，并可能违反 Apple Media Services 条款。",
	},

	backTo: { en: "Back to", zh: "返回" },
	viewAll: { en: "View all {n} storefronts", zh: "查看全部 {n} 个地区" },
	cheapestIn: { en: "cheapest in", zh: "最低价在" },
} satisfies Record<string, Record<Lang, string>>;

export type StringKey = keyof typeof STRINGS;

/** 取一条文案，{name} 占位符用 vars 替换。 */
export function t(lang: Lang, key: StringKey, vars?: Record<string, string | number>): string {
	let s: string = STRINGS[key][lang];
	if (vars) {
		for (const [k, v] of Object.entries(vars)) {
			s = s.replaceAll(`{${k}}`, String(v));
		}
	}
	return s;
}

/** 站内链接。en 是默认语言但也带前缀，避免根路径和 /en 两套 URL 重复收录。 */
export function url(lang: Lang, path = ""): string {
	const clean = path.replace(/^\/+|\/+$/g, "");
	return clean ? `/${lang}/${clean}` : `/${lang}`;
}
