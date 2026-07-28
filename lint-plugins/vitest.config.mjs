import { defineConfig } from "vitest/config";
import { fileURLToPath } from "node:url";

export default defineConfig({
  root: fileURLToPath(new URL(".", import.meta.url)),
  test: {
    name: "lint-plugins",
    environment: "node",
    include: ["__tests__/**/*.test.mjs"],
    passWithNoTests: false,
    coverage: {
      provider: "v8",
      include: ["compozy-design-system.mjs", "ui-primitive-reuse.mjs", "data-boundaries.mjs"],
      exclude: ["vitest.config.mjs", "__tests__/**"],
      reporter: ["text", "json"],
    },
  },
});
