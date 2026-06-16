import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		// Proxy API + live WebSocket to the ridgelined daemon during dev.
		proxy: {
			'/api': {
				target: 'http://localhost:8080',
				ws: true
			}
		}
	}
});
