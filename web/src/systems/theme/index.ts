// Types
export type { ResolvedTheme, ThemePreference } from "./types";

// Runtime
export { applyTheme } from "./lib/apply-theme";
export { installThemeRuntime } from "./lib/install-theme-runtime";
export {
  DEFAULT_THEME_PREFERENCE,
  THEME_PREFERENCES,
  THEME_STORAGE_KEY,
} from "./lib/theme-preference";
export { themePreferenceStore } from "./stores/theme-preference-store";

export {
  DEFAULT_FONT_APPEARANCE,
  FONT_CATALOG,
  cssFontFamily,
  fontLabel,
  isStorableFontFamily,
  type FontAppearance,
  type FontCatalogEntry,
  type FontRole,
} from "./lib/font-appearance";
export {
  queryInstalledFonts,
  supportsInstalledFonts,
  type InstalledFonts,
} from "./lib/installed-fonts";

// Components
export { FontAppearanceSync } from "./components/font-appearance-sync";
export { ThemeToaster } from "./components/theme-toaster";

// Hooks
export { useFontAppearance, type FontAppearanceHandle } from "./hooks/use-font-appearance";
export { useThemePreference, type ThemePreferenceHandle } from "./hooks/use-theme-preference";
