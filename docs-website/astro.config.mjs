// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
	site: 'https://filesystem-light.scopweb.com',
	integrations: [
		starlight({
			expressiveCode: {
				themes: ['starlight-dark', 'starlight-light'],
			},
			head: [
				{ tag: 'link', attrs: { rel: 'preconnect', href: 'https://fonts.googleapis.com' } },
				{ tag: 'link', attrs: { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: true } },
				{
					tag: 'link',
					attrs: {
						rel: 'stylesheet',
						href: 'https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,300;0,9..40,400;0,9..40,500;0,9..40,600;1,9..40,400&family=Space+Mono:ital,wght@0,400;0,700;1,400&display=swap',
					},
				},
			],
			title: 'MCP Filesystem Server',
			description: 'Lightweight MCP filesystem server for Claude Desktop with normalizer intelligence',
			social: [
				{ icon: 'github', label: 'GitHub', href: 'https://github.com/scopweb/mcp-filesystem-go-light' },
				{ icon: 'external', label: 'scopweb.com', href: 'https://scopweb.com' },
			],
			defaultLocale: 'root',
			locales: { root: { label: 'English', lang: 'en' } },
			sidebar: [
				{
					label: 'Getting Started',
					items: [
						{ label: 'Installation', slug: 'getting-started/installation' },
						{ label: 'Claude Desktop Setup', slug: 'getting-started/claude-desktop-setup' },
					],
				},
				{
					label: 'Features',
					items: [
						{ label: 'Tools Reference', slug: 'features/tools' },
						{ label: 'Request Normalizer', slug: 'features/normalizer' },
						{ label: 'Security', slug: 'features/security' },
					],
				},
				{
					label: 'Reference',
					items: [
						{ label: 'vs Ultra', slug: 'reference/vs-ultra' },
						{ label: 'Changelog', slug: 'reference/changelog' },
					],
				},
			],
			customCss: ['./src/styles/custom.css'],
		}),
	],
});
