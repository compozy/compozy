import type { StorybookConfig } from "@storybook/react-vite";
import { fileURLToPath } from "node:url";

const config: StorybookConfig = {
  stories: ["../src/**/*.stories.@(ts|tsx)"],
  addons: ["@storybook/addon-docs", "@storybook/addon-a11y", "@storybook/addon-themes"],
  staticDirs: ["../public"],
  framework: {
    name: "@storybook/react-vite",
    // The shared rendering config, without the app's `proto()` production boundary.
    options: {
      builder: {
        viteConfigPath: fileURLToPath(new URL("../vite.presentation.ts", import.meta.url)),
      },
    },
  },
};

export default config;
