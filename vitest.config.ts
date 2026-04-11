import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    pool: "threads",
    isolate: true,
    passWithNoTests: true,
    exclude: ["**/node_modules/**", "**/dist/**", "**/bin/**"],
    coverage: {
      provider: "v8",
      reporter: ["text", "json", "html"],
      exclude: ["**/node_modules/**", "**/dist/**", "**/*.d.ts", "**/*.config.*"],
    },
  },
});
