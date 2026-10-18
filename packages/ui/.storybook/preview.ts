import type { Preview } from "@storybook/react-vite";
import { withThemeByClassName, withThemeByDataAttribute } from "@storybook/addon-themes";
import { createElement, type ReactNode } from "react";

import "./preview.css";
import { UIProvider } from "../src/components/custom/ui-provider";

type StoryRenderer = () => ReactNode;

// The app theme contract: `data-theme` and `.dark` move together on <html>.
// Both decorators read the same toolbar `theme` global, so they never diverge.
export const themeAttributeDecorator = withThemeByDataAttribute({
  themes: {
    light: "light",
    dark: "dark",
  },
  defaultTheme: "dark",
  attributeName: "data-theme",
  parentSelector: "html",
});

export const themeClassDecorator = withThemeByClassName({
  themes: {
    light: "",
    dark: "dark",
  },
  defaultTheme: "dark",
  parentSelector: "html",
});

export const uiProviderDecorator = (Story: StoryRenderer) =>
  createElement(UIProvider, null, createElement(Story));

export const storybookDecorators = [
  themeAttributeDecorator,
  themeClassDecorator,
  uiProviderDecorator,
];

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
