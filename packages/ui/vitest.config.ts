import babel from "@rolldown/plugin-babel";
import viteReact, { reactCompilerPreset } from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  // Consumers compile these sources with the React Compiler; tests run the same
  // transform so memoized output that ignores changing inputs fails here too.
  plugins: [viteReact(), babel({ presets: [reactCompilerPreset()] })],
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
