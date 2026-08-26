import tailwindcss from "@tailwindcss/vite";
import type { StorybookConfig } from "@storybook/react-vite";
import { mergeConfig } from "vite";

const config: StorybookConfig = {
  stories: ["../src/**/*.stories.@(ts|tsx)"],
  // Serves self-hosted Emojibase data at /vendor/emojibase for SymbolPicker stories.
  staticDirs: [{ from: "../node_modules/emojibase-data", to: "/vendor/emojibase" }],
  addons: ["@storybook/addon-docs", "@storybook/addon-a11y", "@storybook/addon-themes"],
  framework: {
    name: "@storybook/react-vite",
    options: {},
  },
  viteFinal: async config =>
    mergeConfig(config, {
      plugins: [tailwindcss()],
    }),
};

export default config;
