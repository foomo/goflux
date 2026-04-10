import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
	title: 'goflux',
	description: 'Type-safe pub/sub messaging for Go',
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
			{ text: 'Examples', link: '/examples/' },
			{ text: 'API', link: '/api/' },
		],
		sidebar: [
			{
				text: 'Guide',
				items: [
					{ text: 'Introduction', link: '/guide/introduction' },
					{ text: 'Getting Started', link: '/guide/getting-started' },
					{ text: 'Core Concepts', link: '/guide/core-concepts' },
					{ text: 'Transports', link: '/guide/transports' },
					{ text: 'Pipelines', link: '/guide/pipelines' },
					{ text: 'Middleware', link: '/guide/middleware' },
					{ text: 'Distribution', link: '/guide/distribution' },
					{ text: 'Telemetry', link: '/guide/telemetry' },
				],
			},
			{
				text: 'Examples',
				items: [
					{ text: 'Overview', link: '/examples/' },
					{ text: 'Basic Pub/Sub', link: '/examples/basic-pubsub' },
					{ text: 'NATS Pipeline', link: '/examples/nats-pipeline' },
					{ text: 'HTTP Webhook', link: '/examples/http-webhook' },
					{ text: 'Fan Patterns', link: '/examples/fan-patterns' },
					{ text: 'Middleware Chain', link: '/examples/middleware-chain' },
				],
			},
			{
				text: 'API',
				items: [
					{ text: 'Reference', link: '/api/' },
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
				content: 'Type-safe pub/sub messaging for Go',
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
