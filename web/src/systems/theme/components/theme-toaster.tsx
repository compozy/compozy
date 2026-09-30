import { Toaster, type ToasterProps } from "@compozy/ui";

import { useThemePreference } from "../hooks/use-theme-preference";

/** The app's toast region, painted in the resolved theme. */
export function ThemeToaster(props: Omit<ToasterProps, "theme">) {
  const { resolvedTheme } = useThemePreference();
  return <Toaster {...props} theme={resolvedTheme} />;
}
