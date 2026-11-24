// SPDX-License-Identifier: FSL-1.1-MIT

import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    name: "cloud-edge",
    environment: "node",
    include: ["__tests__/**/*.test.ts"],
  },
});
