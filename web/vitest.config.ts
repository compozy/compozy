import tailwindcss from "@tailwindcss/vite";
import viteReact from "@vitejs/plugin-react";
import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [viteReact(), tailwindcss()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  test: {
    name: "web",
    environment: "jsdom",
    globals: true,
    // Bounded pool: turbo runs workspace test tasks concurrently (L-030).
    maxWorkers: "50%",
    // Several UI integration suites legitimately exceed Vitest's default timeout
    // under full-suite load; use an explicit budget for stable CI/local verification.
    testTimeout: 20_000,
    include: ["src/**/*.{test,spec}.{ts,tsx}", "e2e/**/*.test.ts", "tests/**/*.test.{ts,tsx}"],
    exclude: ["**/node_modules/**", "**/dist/**", "tests/visual/*.spec.ts"],
    setupFiles: ["./src/test-setup.ts"],
  },
});
