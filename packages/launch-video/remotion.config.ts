import path from "node:path";

import { Config } from "@remotion/cli/config";

Config.setVideoImageFormat("jpeg");
Config.setOverwriteOutput(true);
Config.setEntryPoint("./src/index.ts");

// Remotion's bundler does not read tsconfig `paths`; mirror the `@/*` alias the source uses.
Config.overrideWebpackConfig(config => ({
  ...config,
  resolve: {
    ...config.resolve,
    alias: {
      ...config.resolve?.alias,
      "@": path.resolve(process.cwd(), "src"),
    },
  },
}));
