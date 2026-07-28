import viteReact from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [viteReact()],
  test: {
    name: "ui",
    environment: "jsdom",
    globals: true,
    // Bounded pool: turbo runs workspace test tasks concurrently (L-030).
    maxWorkers: "50%",
    include: ["src/**/*.{test,spec}.{ts,tsx}", "tests/**/*.test.{ts,tsx}"],
    exclude: ["**/node_modules/**", "**/dist/**", "tests/visual/*.spec.ts"],
    setupFiles: ["./src/test-setup.ts"],
  },
});
