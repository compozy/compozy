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

// Hooks
export { useThemePreference, type ThemePreferenceHandle } from "./hooks/use-theme-preference";
