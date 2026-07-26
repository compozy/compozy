import react from "@vitejs/plugin-react";
import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vitest/config";

const siteRoot = fileURLToPath(new URL(".", import.meta.url));

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL(".", import.meta.url)),
      "@agh/ui/utils": fileURLToPath(new URL("../ui/src/lib/utils.ts", import.meta.url)),
      "@agh/ui/tokens.css": fileURLToPath(new URL("../ui/src/tokens.css", import.meta.url)),
      "@agh/ui": fileURLToPath(new URL("../ui/src", import.meta.url)),
      "#site/content": fileURLToPath(new URL(".velite/index.js", import.meta.url)),
    },
  },
  test: {
    name: "site",
    environment: "jsdom",
    // Bounded pool: turbo runs workspace test tasks concurrently (L-030).
    maxWorkers: "50%",
    setupFiles: ["./vitest.setup.tsx"],
    env: {
      AGH_SITE_ROOT: siteRoot,
    },
    globals: true,
    include: ["**/*.{test,spec}.{ts,tsx}"],
    exclude: ["**/node_modules/**", "**/out/**", "**/.next/**"],
  },
});
