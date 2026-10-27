/**
 * Appearance fonts (#984). The operator's Interface and Code font live in the
 * daemon's `[shell.appearance]` section so every client of the same CompozyOS
 * paints with them; this module turns a stored family name into the
 * `--user-font-*` variables that head the `--font-sans` / `--font-mono`
 * stacks in `packages/ui/src/tokens.css`. The built-in stacks stay in CSS, so
 * a family missing on this device falls through to them.
 */

export type FontRole = "sans" | "mono";

/** One family name per role; `""` keeps the built-in stack (Inter / Geist Mono). */
export type FontAppearance = Record<FontRole, string>;

export const DEFAULT_FONT_APPEARANCE: FontAppearance = { sans: "", mono: "" };

/**
 * Local-storage key caching the last applied variables for the pre-paint boot
 * script `web/public/theme-boot.js` — change both together.
 */
export const FONT_STORAGE_KEY = "compozy.fonts";

export const USER_FONT_VARIABLES: Record<FontRole, string> = {
  sans: "--user-font-sans",
  mono: "--user-font-mono",
};

export interface FontCatalogEntry {
  family: string;
  label: string;
  roles: readonly FontRole[];
}

/**
 * Curated choices offered on every device. JetBrains Mono ships with the app
 * but only downloads when chosen; the System entries resolve the platform UI
 * and monospace faces without any download.
 */
export const FONT_CATALOG: readonly FontCatalogEntry[] = [
  { family: "JetBrains Mono Variable", label: "JetBrains Mono", roles: ["sans", "mono"] },
  { family: "system-ui", label: "System", roles: ["sans"] },
  { family: "ui-monospace", label: "System", roles: ["mono"] },
];

/** Built-in faces each role falls back to, named for the Default entry. */
export const DEFAULT_FONT_LABEL: Record<FontRole, string> = {
  sans: "Inter",
  mono: "Geist Mono",
};

const BUNDLED_FONT_LOADERS: Record<string, () => Promise<unknown>> = {
  "JetBrains Mono Variable": () => import("@fontsource-variable/jetbrains-mono"),
};

/** CSS generic keywords — written bare; quoting turns them into family names. */
const GENERIC_FAMILIES = new Set([
  "monospace",
  "sans-serif",
  "serif",
  "system-ui",
  "ui-monospace",
  "ui-sans-serif",
  "ui-serif",
  "ui-rounded",
]);

/**
 * Mirrors the daemon's `shell.appearance.*` validation (`internal/config/
 * shell_appearance.go`): letters, digits, space and `-_.'`, no outer spaces,
 * at most 128 characters. Installed families outside it are not offered.
 */
const STORABLE_FAMILY = /^[\p{L}\p{Nd}_.'-](?:[\p{L}\p{Nd} _.'-]*[\p{L}\p{Nd}_.'-])?$/u;
const MAX_FAMILY_LENGTH = 128;

export function isStorableFontFamily(family: string): boolean {
  return [...family].length <= MAX_FAMILY_LENGTH && STORABLE_FAMILY.test(family);
}

export function fontLabel(family: string, role: FontRole): string {
  if (family === "") return DEFAULT_FONT_LABEL[role];
  const entry = FONT_CATALOG.find(item => item.family === family && item.roles.includes(role));
  return entry?.label ?? family;
}

/** The `font-family` value for one family name (also used for previews). */
/**
 * Chromium (and so the desktop app) does not implement the `ui-monospace`
 * generic, so the System Code font names the platform faces it stands for.
 */
const SYSTEM_FAMILY_STACKS: Record<string, string> = {
  "ui-monospace": 'ui-monospace, "SF Mono", "SFMono-Regular", Menlo, Consolas, "Liberation Mono"',
};

export function cssFontFamily(family: string): string {
  const stack = SYSTEM_FAMILY_STACKS[family];
  if (stack) return stack;
  if (GENERIC_FAMILIES.has(family)) return family;
  return `"${family.replace(/["\\]/g, "")}"`;
}

/** A picker preview: the family, then the role's generic so it never renders serif. */
export function previewFontFamily(family: string, role: FontRole): string {
  return `${cssFontFamily(family)}, ${role === "mono" ? "monospace" : "sans-serif"}`;
}

/** Fetches every bundled catalog face so the picker previews them truthfully. */
export function preloadCatalogFonts(): Promise<void> {
  return loadBundledFonts({ sans: "", mono: "" }, Object.keys(BUNDLED_FONT_LOADERS));
}

/**
 * Points `--user-font-*` at the chosen families; an empty choice removes the
 * override so the stack's own first face (the token default) applies again.
 */
export function applyFontAppearance(root: HTMLElement, appearance: FontAppearance): void {
  for (const role of ["sans", "mono"] as const) {
    const family = appearance[role].trim();
    if (family === "") root.style.removeProperty(USER_FONT_VARIABLES[role]);
    else root.style.setProperty(USER_FONT_VARIABLES[role], cssFontFamily(family));
  }
}

/** Starts the download of any bundled face the appearance names. */
export function loadBundledFonts(
  appearance: FontAppearance,
  extra: readonly string[] = []
): Promise<void> {
  const loads = new Set(
    [appearance.sans, appearance.mono, ...extra].flatMap(family => {
      const loader = BUNDLED_FONT_LOADERS[family];
      return loader ? [loader] : [];
    })
  );
  return Promise.all([...loads].map(load => load())).then(
    () => undefined,
    (error: unknown) => console.warn("Failed to load the selected font", error)
  );
}

/**
 * Caches the applied variables for the boot script. The cache stores CSS
 * values, not family names, so the script stays a dumb property copy.
 */
export function writeFontAppearanceCache(appearance: FontAppearance): void {
  if (typeof window === "undefined") return;
  try {
    const cached: Record<string, string> = {};
    for (const role of ["sans", "mono"] as const) {
      const family = appearance[role].trim();
      if (family !== "") cached[USER_FONT_VARIABLES[role]] = cssFontFamily(family);
    }
    if (Object.keys(cached).length === 0) window.localStorage.removeItem(FONT_STORAGE_KEY);
    else window.localStorage.setItem(FONT_STORAGE_KEY, JSON.stringify(cached));
  } catch (error) {
    console.warn("Failed to cache the font appearance", error);
  }
}
