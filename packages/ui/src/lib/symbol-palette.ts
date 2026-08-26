/**
 * Value shapes and matching rules for `SymbolPicker`.
 *
 * Domain-free on purpose: the picker knows how to search, tint, and validate,
 * while the icon set, emoji set, and suggested swatches are supplied by whoever
 * mounts it.
 */
import { formatHexColor, parseHexColor } from "./contrast";

export type SymbolKind = "icon" | "emoji";

export interface SymbolPickerLabels {
  kind: string;
  icons: string;
  emojis: string;
  searchIcons: string;
  searchEmojis: string;
  swatches: string;
  customColor: string;
  customColorToggle: string;
  invalidColor: string;
  skinTone: string;
  loadingEmojis: string;
  loadingIcons: string;
  noResults: (kind: SymbolKind, query: string, otherTab: string) => string;
}

/** Generic English. Override to localize or to re-word for a surface. */
export const SYMBOL_PICKER_DEFAULT_LABELS: SymbolPickerLabels = {
  kind: "Symbol kind",
  icons: "Icons",
  emojis: "Emojis",
  searchIcons: "Search icons",
  searchEmojis: "Search emojis",
  swatches: "Suggested colors",
  customColor: "Custom color",
  customColorToggle: "Pick a custom color",
  invalidColor: "Enter a color like #4ea7fc.",
  skinTone: "Change skin tone",
  loadingEmojis: "Loading emojis…",
  loadingIcons: "Loading icons…",
  noResults: (kind, query, otherTab) =>
    `No ${kind === "icon" ? "icons" : "emojis"} match "${query}". Try the ${otherTab} tab.`,
};

export interface SymbolValue {
  kind: SymbolKind;
  /** An icon token for `kind: "icon"`, or the grapheme itself for `kind: "emoji"`. */
  value: string;
}

export interface SymbolSwatch {
  label: string;
  value: string;
}

export interface SymbolIconOption {
  /** Sprite symbol id, also the accessible name when no label is supplied. */
  name: string;
  label?: string;
  keywords?: string;
}

/**
 * Canonicalizes a user-typed color to `#rrggbb`, accepting shorthand and a
 * missing hash. Returns null when the value is not a color, which is what the
 * picker's inline error branches on.
 */
export function normalizeHexColor(input: string): string | null {
  const parsed = parseHexColor(input);
  return parsed === null ? null : formatHexColor(parsed);
}

/** Case-insensitive substring match over name, label, and keywords. */
export function matchesSymbolQuery(option: SymbolIconOption, query: string): boolean {
  const needle = query.trim().toLowerCase();
  if (needle === "") return true;
  const haystack = `${option.name} ${option.label ?? ""} ${option.keywords ?? ""}`.toLowerCase();
  return haystack.includes(needle);
}
