import { defineConfig } from "vite";
import wails from "@wailsio/runtime/plugins/vite";

export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  build: {
    sourcemap: false,
    assetsInlineLimit: 0,
    modulePreload: { polyfill: false },
  },
  plugins: [wails("./bindings")],
});
