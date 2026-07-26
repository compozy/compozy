import type { Preview } from "@storybook/react-vite";
import { withThemeByClassName } from "@storybook/addon-themes";
import { createElement, type ReactNode } from "react";

import "./preview.css";
import { UIProvider } from "../src/components/custom/ui-provider";

type StoryRenderer = () => ReactNode;

export const themeDecorator = withThemeByClassName({
  themes: {
    light: "",
    dark: "dark",
  },
  defaultTheme: "dark",
});

export const uiProviderDecorator = (Story: StoryRenderer) =>
  createElement(UIProvider, null, createElement(Story));

export const storybookDecorators = [themeDecorator, uiProviderDecorator];

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
