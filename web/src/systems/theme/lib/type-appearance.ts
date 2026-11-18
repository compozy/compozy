/**
 * Appearance text size and letter spacing (`[shell.appearance]` `text_size` /
 * `letter_spacing`): a few named steps rather than free numbers, so every
 * choice keeps the type scale's proportions. Size sets `--user-text-scale`,
 * which multiplies every rem `--text-*` step and its leading in
 * `packages/ui/src/tokens.css`; spacing sets `--user-tracking-body`, which
 * feeds the body tracking. The terminal sizes itself apart.
 */
import type { components } from "@compozy/client";

type ShellAppearance = components["schemas"]["SettingsShellAppearancePayload"];

export type TextSize = NonNullable<ShellAppearance["text_size"]>;
export type LetterSpacing = NonNullable<ShellAppearance["letter_spacing"]>;

export interface TypeAppearance {
  size: TextSize;
  spacing: LetterSpacing;
}

export const DEFAULT_TYPE_APPEARANCE: TypeAppearance = { size: "default", spacing: "default" };

export interface TypeStep<T extends string> {
  value: T;
  label: string;
}

export const TEXT_SIZE_STEPS: readonly (TypeStep<TextSize> & { scale: string })[] = [
  { value: "small", label: "Small", scale: "0.95" },
  { value: "default", label: "Default", scale: "1" },
  { value: "large", label: "Large", scale: "1.075" },
  { value: "larger", label: "Larger", scale: "1.15" },
];

export const LETTER_SPACING_STEPS: readonly (TypeStep<LetterSpacing> & { tracking: string })[] = [
  { value: "tight", label: "Tight", tracking: "-0.02em" },
  { value: "default", label: "Default", tracking: "-0.01em" },
  { value: "relaxed", label: "Relaxed", tracking: "0em" },
  { value: "wide", label: "Wide", tracking: "0.01em" },
];

/**
 * Local-storage key caching the last applied variables for the pre-paint boot
 * script `web/public/theme-boot.js` — change both together.
 */
export const TYPE_STORAGE_KEY = "compozy.type";

export const TEXT_SCALE_VARIABLE = "--user-text-scale";
export const TRACKING_VARIABLE = "--user-tracking-body";

export function isTextSize(value: unknown): value is TextSize {
  return TEXT_SIZE_STEPS.some(step => step.value === value);
}

export function isLetterSpacing(value: unknown): value is LetterSpacing {
  return LETTER_SPACING_STEPS.some(step => step.value === value);
}

/** The CSS overrides one choice publishes; the default steps publish none. */
function typeVariables(type: TypeAppearance): Record<string, string | null> {
  const size = TEXT_SIZE_STEPS.find(step => step.value === type.size);
  const spacing = LETTER_SPACING_STEPS.find(step => step.value === type.spacing);
  return {
    [TEXT_SCALE_VARIABLE]: size && size.value !== "default" ? size.scale : null,
    [TRACKING_VARIABLE]: spacing && spacing.value !== "default" ? spacing.tracking : null,
  };
}

/** Points the type variables at a choice; a default step removes its override. */
export function applyTypeAppearance(root: HTMLElement, type: TypeAppearance): void {
  for (const [name, value] of Object.entries(typeVariables(type))) {
    if (value === null) root.style.removeProperty(name);
    else root.style.setProperty(name, value);
  }
}

/** Caches the applied variables for the next boot; the boot script copies them verbatim. */
export function writeTypeAppearanceCache(type: TypeAppearance): void {
  if (typeof window === "undefined") return;
  try {
    const cached = Object.fromEntries(
      Object.entries(typeVariables(type)).filter(([, value]) => value !== null)
    );
    if (Object.keys(cached).length === 0) window.localStorage.removeItem(TYPE_STORAGE_KEY);
    else window.localStorage.setItem(TYPE_STORAGE_KEY, JSON.stringify(cached));
  } catch (error) {
    console.warn("Failed to cache the text appearance", error);
  }
}
