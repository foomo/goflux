import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
	title: 'goflux',
	description: 'Generic, transport-agnostic messaging patterns for Go',
	lang: "en-US",
	lastUpdated: true,
	appearance: "dark",
	ignoreDeadLinks: false,
	base: '/goflux/',
	sitemap: {
		hostname: 'https://foomo.github.io/goflux',
	},
	themeConfig: {
		// https://vitepress.dev/reference/default-theme-config
		logo: '/logo.png',
		outline: [2, 4],
		nav: [
			{ text: 'Guide', link: '/guide/introduction' },
			{ text: 'Patterns', link: '/guide/patterns/fire-and-forget' },
			{ text: 'Transports', link: '/transports/channel' },
			{ text: 'Reference', link: '/reference/' },
		],
		sidebar: [
			{
				text: 'Guide',
				items: [
					{ text: 'Introduction', link: '/guide/introduction' },
					{ text: 'Getting Started', link: '/guide/getting-started' },
					{ text: 'Core Concepts', link: '/guide/core-concepts' },
					{ text: 'goflow Integration', link: '/guide/goflow-integration' },
				],
			},
			{
				text: 'Patterns',
				items: [
					{ text: 'Fire & Forget', link: '/guide/patterns/fire-and-forget' },
					{ text: 'At-Least-Once', link: '/guide/patterns/at-least-once' },
					{ text: 'Pull Consumer', link: '/guide/patterns/pull-consumer' },
					{ text: 'Request-Reply', link: '/guide/patterns/request-reply' },
					{ text: 'Queue Groups', link: '/guide/patterns/queue-groups' },
					{ text: 'Fan-Out & Fan-In', link: '/guide/patterns/fan-out-fan-in' },
					{ text: 'Stream Processing', link: '/guide/patterns/stream-processor' },
					{ text: 'Headers', link: '/guide/patterns/headers' },
				],
			},
			{
				text: 'Transports',
				items: [
					{ text: 'Channel', link: '/transports/channel' },
					{ text: 'NATS', link: '/transports/nats' },
					{ text: 'JetStream', link: '/transports/jetstream' },
					{ text: 'HTTP', link: '/transports/http' },
				],
			},
			{
				text: 'Composition',
				items: [
					{ text: 'Middleware', link: '/middleware/' },
					{ text: 'Pipeline', link: '/pipeline/' },
				],
			},
			{
				text: 'Observability',
				items: [
					{ text: 'Telemetry', link: '/telemetry/' },
				],
			},
			{
				text: 'API',
				items: [
					{ text: 'Reference', link: '/reference/' },
				],
			},
			{
				text: 'Contributing',
				collapsed: true,
				items: [
					{
						text: "Guideline",
						link: '/CONTRIBUTING.md',
					},
					{
						text: "Code of conduct",
						link: '/CODE_OF_CONDUCT.md',
					},
					{
						text: "Security guidelines",
						link: '/SECURITY.md',
					},
				],
			},
		],
		socialLinks: [
			{ icon: 'github', link: 'https://github.com/foomo/goflux' },
		],
		editLink: {
			pattern: 'https://github.com/foomo/goflux/edit/main/docs/:path',
		},
		search: {
			provider: 'local',
		},
		footer: {
			message: 'Made with ♥ <a href="https://www.foomo.org">foomo</a> by <a href="https://www.bestbytes.com">bestbytes</a>',
		},
	},
	markdown: {
		// https://github.com/vuejs/vitepress/discussions/3724
		theme: {
			light: 'catppuccin-latte',
			dark: 'catppuccin-frappe',
		}
	},
	head: [
		['meta', { name: 'theme-color', content: '#ffffff' }],
		['link', { rel: 'icon', href: '/logo.png' }],
		['meta', { name: 'author', content: 'foomo by bestbytes' }],
		// OpenGraph
		['meta', { property: 'og:title', content: 'foomo/goflux' }],
		[
			'meta',
			{
				property: 'og:image',
				content: 'https://github.com/foomo/goflux/blob/main/docs/public/banner.png?raw=true',
			},
		],
		[
			'meta',
			{
				property: 'og:description',
				content: 'Generic, transport-agnostic messaging patterns for Go',
			},
		],
		['meta', { name: 'twitter:card', content: 'summary_large_image' }],
		[
			'meta',
			{
				name: 'twitter:image',
				content: 'https://github.com/foomo/goflux/blob/main/docs/public/banner.png?raw=true',
			},
		],
		[
			'meta', { name: 'viewport', content: 'width=device-width, initial-scale=1.0, viewport-fit=cover',
			},
		],
	]
})
