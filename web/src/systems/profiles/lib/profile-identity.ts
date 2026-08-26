import spriteUrl from "lucide-static/sprite.svg?url";

import type { SymbolIconOption, SymbolSwatch, SymbolValue } from "@compozy/ui";

import type { ProfilePayload } from "../types";

/** Sprite that renders every Lucide icon slug the daemon accepts. */
export const PROFILE_SPRITE_URL = spriteUrl;

/** Emojibase JSON copied into the build by vite.config.ts (works offline). */
export const PROFILE_EMOJIBASE_URL = "/vendor/emojibase";

/**
 * The full Lucide catalog with search keywords, loaded lazily so no icon data
 * rides the app bundle. The daemon validates stored slugs against the same
 * lucide-static release (internal/profile/lucide_icons.gen.txt).
 */
export async function loadProfileIconCatalog(): Promise<SymbolIconOption[]> {
  // Hand-typed so tsc never infers the 1.8k-entry JSON literal.
  const { default: tags } = (await import("lucide-static/tags.json")) as unknown as {
    default: Record<string, readonly string[]>;
  };
  return Object.entries(tags).map(([name, keywords]) => ({
    name,
    label: name.replace(/-/g, " "),
    keywords: keywords.join(" "),
  }));
}

/**
 * Suggested colors, deliberately clear of the signal hexes so an identity can
 * never be mistaken for a status. A typed value may still land on one — that is
 * the operator's data, and the picker accepts it.
 */
export const PROFILE_IDENTITY_SWATCHES: readonly SymbolSwatch[] = [
  { label: "Gray", value: "#8a8f98" },
  { label: "Blue", value: "#4ea7fc" },
  { label: "Teal", value: "#2aa8a8" },
  { label: "Green", value: "#4cb782" },
  { label: "Violet", value: "#c26ad6" },
  { label: "Pink", value: "#e56aa2" },
  { label: "Amber", value: "#e8b04a" },
  { label: "Brown", value: "#b58e5f" },
];

const STARTER_ICONS = [
  "compass",
  "rocket",
  "leaf",
  "sparkles",
  "target",
  "flame",
  "gem",
  "anchor",
] as const;

/**
 * A profile is never born blank (US-001.AC-3): creation pre-picks a distinct
 * pairing so it is recognisable from its first second. The pick rotates off the
 * count of profiles that already exist, so two profiles made back to back do not
 * look alike.
 */
export function starterIdentity(existingCount: number): { color: string; icon: string } {
  const index = Math.max(0, existingCount);
  const swatch = PROFILE_IDENTITY_SWATCHES[index % PROFILE_IDENTITY_SWATCHES.length];
  return {
    color: swatch.value,
    icon: STARTER_ICONS[index % STARTER_ICONS.length],
  };
}

/** The symbol a stored profile renders, normalized for the picker. */
export function symbolOf(profile: Pick<ProfilePayload, "icon" | "emoji">): SymbolValue {
  const emoji = profile.emoji?.trim();
  if (emoji) return { kind: "emoji", value: emoji };
  const icon = profile.icon?.trim();
  return { kind: "icon", value: icon && icon !== "" ? icon : "user-round" };
}

/** Turns a picked symbol back into the mutually exclusive wire fields. */
export function symbolPatch(symbol: SymbolValue): { icon?: string; emoji?: string } {
  return symbol.kind === "emoji" ? { emoji: symbol.value } : { icon: symbol.value };
}
