import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Build the SPA into the Go embed dir. base: "./" keeps asset paths relative
// so go:embed serves them under any host/port.
export default defineConfig({
  plugins: [react()],
  base: "./",
  server: {
    proxy: { "/api": "http://127.0.0.1:7777" },
  },
  build: {
    outDir: "../internal/ui/web/static",
    emptyOutDir: true,
  },
});
