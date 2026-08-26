// 读取 `aiprice export` 产出的 JSON，并提供页面需要的派生数据。
//
// 全部在构建期跑，运行时没有 JS 取数——站点是纯静态的。

import metaJson from "../data/meta.json";
import productsJson from "../data/products.json";
import storefrontsJson from "../data/storefronts.json";
import chatgpt from "../data/prices/chatgpt.json";
import claude from "../data/prices/claude.json";
import gemini from "../data/prices/gemini.json";
import grok from "../data/prices/grok.json";
import historyJson from "../data/history.json";
import { type Lang } from "./i18n";

export interface Meta {
	dataDate: string;
	fxDate: string;
	fxSource: string;
	generatedAt: string;
	storefronts: number;
	products: number;
	cnyPerUsd: number;
	priceCount: number;
	days: number;
}

export interface Storefront {
	code: string;
	currency: string;
	flag: string;
	nameEn: string;
	nameZh: string;
	currencyEn: string;
	currencyZh: string;
}

export interface Tier {
	productId: string;
	tier: string;
	tierEn: string;
	tierZh: string;
	period: string;
	featured: boolean;
}

export interface Product {
	id: string;
	nameEn: string;
	nameZh: string;
	vendor: string;
	appStoreId: number;
	bundleId: string;
	tiers: Tier[];
}

export interface PriceRow {
	storefront: string;
	currency: string;
	rawPrice: string;
	amount: number;
	usd: number;
}

export interface TierPrices {
	tier: string;
	tierEn: string;
	tierZh: string;
	period: string;
	rows: PriceRow[]; // 已按美元价升序
}

export interface HistoryPoint {
	date: string;
	usd: number;
	storefront: string;
}

export interface HistorySeries {
	productId: string;
	tier: string;
	tierEn: string;
	tierZh: string;
	period: string;
	points: HistoryPoint[];
}

export interface PriceChange {
	date: string;
	prevDate: string;
	productId: string;
	tier: string;
	tierEn: string;
	tierZh: string;
	period: string;
	storefront: string;
	currency: string;
	fromRaw: string;
	toRaw: string;
	pct: number;
}

export interface History {
	days: string[];
	series: HistorySeries[];
	changes: PriceChange[];
}

export const history = historyJson as History;

/** 首页那四个代表档位的历史序列，用于趋势图。 */
export function headlineSeries(): HistorySeries[] {
	const out: HistorySeries[] = [];
	for (const p of products) {
		const tp = headlineTier(p);
		if (!tp) continue;
		const s = history.series.find(
			(x) => x.productId === p.id && x.tier === tp.tier && x.period === tp.period,
		);
		if (s && s.points.length > 0) out.push(s);
	}
	return out.sort((a, b) => {
		const la = a.points[a.points.length - 1]?.usd ?? 0;
		const lb = b.points[b.points.length - 1]?.usd ?? 0;
		return lb - la;
	});
}

export const meta = metaJson as Meta;
export const products = productsJson as Product[];
export const storefronts = storefrontsJson as Storefront[];

const PRICES: Record<string, TierPrices[]> = {
	chatgpt: chatgpt as TierPrices[],
	claude: claude as TierPrices[],
	gemini: gemini as TierPrices[],
	grok: grok as TierPrices[],
};

/** 产品在设计稿里的序列色，和 global.css 的 --s1..--s4 对应 */
export const SERIES_VAR: Record<string, string> = {
	chatgpt: "var(--s1)",
	claude: "var(--s2)",
	gemini: "var(--s3)",
	grok: "var(--s4)",
};

const storefrontByCode = new Map(storefronts.map((s) => [s.code, s]));

export function storefrontOf(code: string): Storefront | undefined {
	return storefrontByCode.get(code);
}

export function regionName(s: Storefront, lang: Lang): string {
	return lang === "zh" ? s.nameZh : s.nameEn;
}

export function currencyName(s: Storefront, lang: Lang): string {
	return lang === "zh" ? s.currencyZh : s.currencyEn;
}

export function productName(p: Product, lang: Lang): string {
	return lang === "zh" ? p.nameZh : p.nameEn;
}

export function tierName(t: { tierEn: string; tierZh: string }, lang: Lang): string {
	return lang === "zh" ? t.tierZh : t.tierEn;
}

export function tiersOf(productId: string): TierPrices[] {
	return PRICES[productId] ?? [];
}

/** 取某产品某档位某周期的全球价格表（已按美元升序）。 */
export function tierPrices(productId: string, tier: string, period: string): TierPrices | undefined {
	return tiersOf(productId).find((t) => t.tier === tier && t.period === period);
}

/** 该产品所有「主力档 + 月付」的组合，首页和导航用。 */
export function featuredMonthly(product: Product): TierPrices[] {
	const featured = new Set(product.tiers.filter((t) => t.featured).map((t) => t.tier));
	return tiersOf(product.id)
		.filter((t) => featured.has(t.tier) && t.period === "monthly")
		.sort((a, b) => (a.rows[0]?.usd ?? 0) - (b.rows[0]?.usd ?? 0));
}

/** 产品的代表档位：主力月付里价格居中的那个，用于首页概览。 */
export function headlineTier(product: Product): TierPrices | undefined {
	const list = featuredMonthly(product);
	if (list.length === 0) return undefined;
	// ChatGPT Plus / Claude Pro / AI Pro / SuperGrok 都在 15–35 美元这一档，
	// 取最接近 20 美元的那个作为跨产品可比的代表档。
	return list.reduce((best, cur) => {
		const d = (t: TierPrices) => Math.abs((t.rows[0]?.usd ?? 0) - 20);
		return d(cur) < d(best) ? cur : best;
	});
}

/**
 * 竞赛排名：比它严格便宜的地区数 + 1，并列同名次。
 *
 * 不能用数组下标当排名。实测 158 个地区里有 86 个 ChatGPT Plus 都恰好是 $19.99
 * （美元区共用一个定价），下标排名在这一大坨并列里完全是任意的——同一个美国
 * 可以排到第 11 也可以排到第 96，取决于排序的稳定性。那种数字没有意义。
 */
export interface RankedRow extends PriceRow {
	rank: number;
	/** 与它同价的地区数（含自己）。>1 表示这个名次是并列 */
	tied: number;
}

export function withRanks(rows: PriceRow[]): RankedRow[] {
	const out: RankedRow[] = [];
	let rank = 0;
	let prev = Number.NaN;
	for (let i = 0; i < rows.length; i++) {
		const r = rows[i]!;
		if (!near(r.usd, prev)) {
			rank = i + 1;
			prev = r.usd;
		}
		out.push({ ...r, rank, tied: 0 });
	}
	const counts = new Map<number, number>();
	for (const r of out) counts.set(r.rank, (counts.get(r.rank) ?? 0) + 1);
	for (const r of out) r.tied = counts.get(r.rank) ?? 1;
	return out;
}

/** 价格用浮点存的，比较时留一点容差，避免 19.99 和 19.990000001 被当成不同价 */
function near(a: number, b: number): boolean {
	return Math.abs(a - b) < 0.005;
}

/** 该档位全球一共有多少个不同价位。158 个地区往往只有几十个价位。 */
export function distinctPrices(rows: PriceRow[]): number {
	const seen: number[] = [];
	for (const r of rows) {
		if (!seen.some((v) => near(v, r.usd))) seen.push(r.usd);
	}
	return seen.length;
}

export function rankOf(rows: PriceRow[], code: string): { rank: number; tied: number } | undefined {
	const row = rows.find((r) => r.storefront === code);
	if (!row) return undefined;
	const rank = rows.filter((r) => r.usd < row.usd && !near(r.usd, row.usd)).length + 1;
	const tied = rows.filter((r) => near(r.usd, row.usd)).length;
	return { rank, tied };
}

export interface TierStats {
	lowest: PriceRow;
	highest: PriceRow;
	median: PriceRow;
	us?: PriceRow;
	usRank?: number;
	/** 与美国同价的地区数（含美国）。>1 说明美国这个名次是并列 */
	usTied?: number;
	count: number;
	/** 最高比最低贵多少（0.5 = 最低比最高便宜 50%） */
	spread: number;
}

export function statsOf(tp: TierPrices): TierStats | undefined {
	const rows = tp.rows;
	if (rows.length === 0) return undefined;
	const lowest = rows[0]!;
	const highest = rows[rows.length - 1]!;
	const median = rows[Math.floor(rows.length / 2) - 1] ?? lowest;
	const us = rows.find((r) => r.storefront === "us");
	const usRank = us ? rankOf(rows, "us") : undefined;
	return {
		lowest,
		highest,
		median,
		us,
		usRank: usRank?.rank,
		usTied: usRank?.tied,
		count: rows.length,
		spread: highest.usd > 0 ? 1 - lowest.usd / highest.usd : 0,
	};
}

/** 某地区在售的全部档位，按产品分组。 */
export interface RegionEntry {
	product: Product;
	items: Array<{
		tier: TierPrices;
		row: PriceRow;
		rank: number;
		tied: number;
		total: number;
	}>;
}

export function regionEntries(code: string): RegionEntry[] {
	const out: RegionEntry[] = [];
	for (const product of products) {
		const items: RegionEntry["items"] = [];
		for (const tp of tiersOf(product.id)) {
			const row = tp.rows.find((r) => r.storefront === code);
			if (!row) continue;
			const rk = rankOf(tp.rows, code)!;
			items.push({ tier: tp, row, rank: rk.rank, tied: rk.tied, total: tp.rows.length });
		}
		if (items.length > 0) {
			items.sort((a, b) => a.row.usd - b.row.usd);
			out.push({ product, items });
		}
	}
	return out;
}

/** 有价格数据的地区（有些地区四个产品都没上架，不该生成页面）。 */
export function activeStorefronts(): Storefront[] {
	const seen = new Set<string>();
	for (const p of products) {
		for (const tp of tiersOf(p.id)) {
			for (const r of tp.rows) seen.add(r.storefront);
		}
	}
	return storefronts.filter((s) => seen.has(s.code));
}

// ── 格式化 ────────────────────────────────────────────────

export function usd(v: number): string {
	return v.toFixed(2);
}

export function cny(v: number): string {
	const n = v * meta.cnyPerUsd;
	return n >= 1000 ? Math.round(n).toLocaleString("en-US") : n.toFixed(0);
}

export function pct(v: number): string {
	return `${Math.round(v * 100)}%`;
}

/** 相对最低价贵多少，用于排行表最后一列 */
export function vsLowest(v: number, lowest: number): string {
	if (lowest <= 0 || v === lowest) return "—";
	return `+${Math.round((v / lowest - 1) * 100)}%`;
}
