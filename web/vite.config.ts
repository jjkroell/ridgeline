import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		// Allow access via the ve7kod.ca reverse proxy in addition to
		// localhost. The leading dot permits any ve7kod.ca subdomain.
		allowedHosts: ['.ve7kod.ca'],
		// Proxy API + live WebSocket to the ridgelined daemon during dev.
		// Override the daemon address with RIDGELINE_API when the dev server
		// itself runs on the daemon's default port.
		proxy: {
			'/api': {
				target: process.env.RIDGELINE_API ?? 'http://localhost:8080',
				ws: true
			}
		}
	}
});
