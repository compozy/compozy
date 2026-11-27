import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
    pool: "forks",
    // Windows control-server tests spawn powershell.exe to verify real directory and named-pipe ACLs;
    // cold .NET/AMSI startup on CI runners can exceed Vitest's default 5s timeout.
    testTimeout: 20_000,
    coverage: {
      enabled: true,
      provider: "v8",
      reporter: ["text", "json-summary"],
      thresholds: { lines: 80, functions: 80, statements: 80, branches: 80 },
    },
  },
});
