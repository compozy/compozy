/** The operator's stored theme choice. `system` follows `prefers-color-scheme` live. */
export type ThemePreference = "light" | "dark" | "system";

/** The theme actually painted: what `system` resolves to, or the explicit choice. */
export type ResolvedTheme = "light" | "dark";
