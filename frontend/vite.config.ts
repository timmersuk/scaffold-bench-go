import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  test: {
    environment: "jsdom",
    globals: true,
  },
  build: {
    outDir: "../internal/web/dist",
    emptyOutDir: true,
  },
  server: {
    port: 3000,
    headers: {
      "Cache-Control": "no-store",
    },
  },
});
