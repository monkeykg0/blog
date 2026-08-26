import { defineConfig } from "astro/config";
import sitemap from "@astrojs/sitemap";

export default defineConfig({
	site: "https://aiprice.monkeykgai.com",
	trailingSlash: "never",
	build: { format: "file" },
	integrations: [
		sitemap({
			i18n: { defaultLocale: "en", locales: { en: "en", zh: "zh-Hans" } },
		}),
	],
});
