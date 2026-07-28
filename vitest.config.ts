import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    pool: "threads",
    isolate: true,
    // Bounded pool: unbounded per-project workers multiply with turbo's
    // workspace fan-out and collapse the machine next to other worktrees (L-030).
    maxWorkers: "50%",
    exclude: ["**/node_modules/**", "**/dist/**", "**/bin/**"],
    coverage: {
      provider: "v8",
      reporter: ["text", "json", "html"],
      exclude: ["**/node_modules/**", "**/dist/**", "**/*.d.ts", "**/*.config.*"],
    },
    projects: [
      "web/vitest.config.ts",
      "packages/ui/vitest.config.ts",
      "packages/site/vitest.config.ts",
      "sdk/typescript/vitest.config.ts",
      "sdk/create-extension/vitest.config.ts",
    ],
  },
});
