import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    name: "extension-react",
    environment: "node",
    pool: "forks",
    fileParallelism: false,
    sequence: {
      groupOrder: 1,
    },
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
    exclude: ["dist/**", "**/node_modules/**"],
    coverage: {
      provider: "v8",
      reporter: ["text", "json", "html"],
      exclude: ["dist/**", "**/*.d.ts", "src/index.ts"],
    },
  },
});
