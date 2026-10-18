import type { ResolvedTheme } from "../types";

/**
 * Browser-chrome color per theme: the shell chrome (`--color-rail`) value.
 * Mirrored by `web/public/theme-boot.js`, which runs before any CSS loads.
 */
export const THEME_CHROME_COLOR: Readonly<Record<ResolvedTheme, string>> = {
  dark: "#0a0a0a",
  light: "#f2f2f3",
};

/**
 * The only writer of the document theme: sets `data-theme` and the `.dark`
 * class together, plus `color-scheme` and `<meta name="theme-color">`.
 */
export function applyTheme(
  theme: ResolvedTheme,
  root: HTMLElement = document.documentElement
): void {
  root.dataset.theme = theme;
  root.classList.toggle("dark", theme === "dark");
  root.style.colorScheme = theme;
  root.ownerDocument
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute("content", THEME_CHROME_COLOR[theme]);
}
