module.exports = {
  test: {
    name: "app-sdk",
    environment: "node",
    pool: "forks",
    fileParallelism: false,
    include: ["src/**/*.test.ts"],
    exclude: ["dist/**", "**/node_modules/**"],
    coverage: {
      provider: "v8",
      reporter: ["text", "json", "html"],
      exclude: ["src/generated/**", "src/index.ts", "**/*.d.ts"],
    },
  },
} satisfies import("vitest/config").ViteUserConfig;
