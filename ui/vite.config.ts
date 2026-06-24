import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";
import checker from "vite-plugin-checker";
import "vitest/config";

const apiTarget = process.env.CADDY_UI_API_TARGET || "http://127.0.0.1:9001";

// https://vitejs.dev/config/
export default defineConfig({
	plugins: [
		react(),
		checker({
			// e.g. use TypeScript check
			typescript: true,
		}),
	],
	resolve: {
		tsconfigPaths: true,
	},
	server: {
		host: true,
		port: 5173,
		strictPort: true,
		allowedHosts: true,
		proxy: {
			"/api": {
				target: apiTarget,
				changeOrigin: true,
				rewrite: (path) => path.replace(/^\/api/, "") || "/",
			},
		},
	},
	test: {
		environment: "happy-dom",
		setupFiles: ["./vitest-setup.js"],
	},
	assetsInclude: ["**/*.md", "**/*.png", "**/*.svg"],
});
