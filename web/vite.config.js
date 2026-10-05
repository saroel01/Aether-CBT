import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			preprocess: vitePreprocess(),
			// Kit 3 defaults to hourly version polling; keep the Kit 2 behaviour (no polling).
			version: { pollInterval: 0 },
			adapter: adapter({
				pages: 'build',
				assets: 'build',
				fallback: 'index.html',
				precompress: false,
				strict: true
			})
		})
	],
	server: {
		// Dev-only proxy: forward /api/* to the Go backend so the browser treats every request
		// (including the iSpring exam iframe at /api/exam/content/*) as same-origin to the page.
		// This fixes dev-only cross-origin frame blocking (X-Frame-Options: SAMEORIGIN) and the
		// cross-port SameSite cookie, without weakening the production security headers. Has no
		// effect on the static production build (adapter-static), which is served by the backend
		// itself on a single port.
		proxy: {
			'/api': {
				target: 'http://localhost:3000',
				changeOrigin: true
			}
		}
	}
});
