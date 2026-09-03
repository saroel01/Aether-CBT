import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit()],
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
