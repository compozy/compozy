import type { Decorator, Preview } from "@storybook/react-vite";
import { withThemeByClassName } from "@storybook/addon-themes";

import "./preview.css";
import "@compozy/ui/tokens.css";

type StorybookDecorator = Decorator;

export const themeDecorator: StorybookDecorator = withThemeByClassName({
  themes: {
    light: "",
    dark: "dark",
  },
  defaultTheme: "dark",
});

export const storybookDecorators: StorybookDecorator[] = [themeDecorator];

const preview: Preview = {
  decorators: storybookDecorators,
  parameters: {
    backgrounds: {
      disable: true,
    },
    controls: {
      expanded: true,
    },
  },
};

export default preview;
