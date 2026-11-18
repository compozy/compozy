import viteReact from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [viteReact()],
  test: {
    name: "brand",
    environment: "node",
    // Bounded pool: turbo runs workspace test tasks concurrently (L-030).
    maxWorkers: "50%",
    include: ["src/**/*.test.{ts,tsx}"],
  },
});
