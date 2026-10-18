import type { ResolvedTheme, ThemePreference } from "../types";

/**
 * Local-storage key for the theme preference (D3). Mirrored by the pre-paint
 * boot script `web/public/theme-boot.js` — change both together.
 */
export const THEME_STORAGE_KEY = "compozy.theme";

/** Preference used when nothing (or garbage) is stored (D4). */
export const DEFAULT_THEME_PREFERENCE: ThemePreference = "dark";

export const THEME_PREFERENCES: readonly ThemePreference[] = ["light", "dark", "system"];

const DARK_SCHEME_QUERY = "(prefers-color-scheme: dark)";

export function parseThemePreference(raw: string | null | undefined): ThemePreference {
  return raw === "light" || raw === "dark" || raw === "system" ? raw : DEFAULT_THEME_PREFERENCE;
}

export function readThemePreference(): ThemePreference {
  if (typeof window === "undefined") return DEFAULT_THEME_PREFERENCE;
  try {
    return parseThemePreference(window.localStorage.getItem(THEME_STORAGE_KEY));
  } catch {
    return DEFAULT_THEME_PREFERENCE;
  }
}

export function writeThemePreference(preference: ThemePreference): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(THEME_STORAGE_KEY, preference);
  } catch (error) {
    console.warn("Failed to persist the theme preference", error);
  }
}

export function resolveTheme(
  preference: ThemePreference,
  systemPrefersDark: boolean
): ResolvedTheme {
  if (preference !== "system") return preference;
  return systemPrefersDark ? "dark" : "light";
}

/** Reads the system color scheme (SSR/jsdom safe: no `matchMedia` → light). */
export function getSystemPrefersDark(): boolean {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return false;
  return window.matchMedia(DARK_SCHEME_QUERY).matches;
}

/** Subscribes to system color-scheme changes (SSR/jsdom safe). */
export function subscribeSystemColorScheme(callback: (prefersDark: boolean) => void): () => void {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return () => undefined;
  }
  const mql = window.matchMedia(DARK_SCHEME_QUERY);
  const listener = (event: MediaQueryListEvent) => callback(event.matches);
  mql.addEventListener("change", listener);
  return () => mql.removeEventListener("change", listener);
}
