import type { Preview } from "@storybook/nextjs";
import "../src/app/global.css";
import { withNextThemes } from "./decorators/withNextThemes";
import "./fonts.css";

const preview: Preview = {
  parameters: {
    controls: {
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/i,
      },
    },
    nextjs: {
      appDirectory: true,
    },
    // Remove the backgrounds configuration as themes will handle this
  },
  globalTypes: {
    theme: {
      name: "Theme",
      description: "Global theme for components",
      defaultValue: "dark",
      toolbar: {
        icon: "circlehollow",
        items: [
          { value: "light", title: "Light", icon: "sun" },
          { value: "dark", title: "Dark", icon: "moon" },
        ],
        showName: true,
        dynamicTitle: true,
      },
    },
  },
  decorators: [withNextThemes],
};

export default preview;
