import { DecoratorHelpers } from "@storybook/addon-themes";
import type { Decorator } from "@storybook/nextjs-vite";
import { RootProvider } from "fumadocs-ui/provider";
import React, { useEffect } from "react";

const { initializeThemeState, pluckThemeFromContext } = DecoratorHelpers;

// Component to apply theme class to the HTML element
const ThemeWrapper = ({ theme, children }: { theme: string; children: React.ReactNode }) => {
  useEffect(() => {
    // Apply theme class to the document element
    document.documentElement.className = theme;
    document.documentElement.setAttribute("data-theme", theme);

    // Also ensure the color scheme is set for proper browser behavior
    document.documentElement.style.colorScheme = theme;
  }, [theme]);

  return <>{children}</>;
};

export const withNextThemes: Decorator = (Story, context) => {
  // Initialize theme state with available themes
  initializeThemeState(["light", "dark"], "dark");

  // Get the selected theme from Storybook's toolbar
  const selectedTheme = pluckThemeFromContext(context);
  const { themeOverride } = context.parameters.themes ?? {};
  const theme = themeOverride || selectedTheme || "dark";

  return (
    <ThemeWrapper theme={theme}>
      <div className={theme}>
        <RootProvider>
          <Story />
        </RootProvider>
      </div>
    </ThemeWrapper>
  );
};
