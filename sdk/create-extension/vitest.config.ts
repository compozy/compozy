import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    name: "create-extension",
    environment: "node",
    // Bounded pool: turbo runs workspace test tasks concurrently (L-030).
    maxWorkers: "50%",
    include: ["src/**/*.test.ts"],
    exclude: ["dist/**", "**/node_modules/**"],
  },
});
