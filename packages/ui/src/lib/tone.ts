import type { PillTone } from "../components/custom/pill-types";

/**
 * The single tone → class vocabulary. `pill-variants.ts` is the CVA source of
 * truth for pill chrome; this module is the shared map every other toned
 * surface (status line, live badge, meters, metric values, catalog logos)
 * consumes, so the six signal tones never drift back into hand-rolled Records.
 *
 * Decisions (UI-ALIGNMENT-SPEC §B8):
 * - `text` is toned text on the DARK ramp: the accent tone reads `accent-strong`
 *   (brighter than `accent`, which is reserved for text on an accent tint), and
 *   neutral is always `neutral-ink` — never ad-hoc `muted`/`subtle`.
 * - `bg` is the solid signal hue for bars/dots; `tint` the low-alpha wash;
 *   `solid` the filled swatch + readable ink.
 */
export interface ToneClasses {
  text: string;
  bg: string;
  tint: string;
  solid: string;
}

export const TONE_CLASSES: Record<PillTone, ToneClasses> = {
  neutral: {
    text: "text-neutral-ink",
    bg: "bg-muted",
    tint: "bg-neutral-tint",
    solid: "bg-muted text-canvas",
  },
  accent: {
    text: "text-accent-strong",
    bg: "bg-accent",
    tint: "bg-accent-tint",
    solid: "bg-accent text-accent-ink",
  },
  success: {
    text: "text-success",
    bg: "bg-success",
    tint: "bg-success-tint",
    solid: "bg-success text-canvas",
  },
  warning: {
    text: "text-warning",
    bg: "bg-warning",
    tint: "bg-warning-tint",
    solid: "bg-warning text-canvas",
  },
  danger: {
    text: "text-danger",
    bg: "bg-danger",
    tint: "bg-danger-tint",
    solid: "bg-danger text-canvas",
  },
  info: { text: "text-info", bg: "bg-info", tint: "bg-info-tint", solid: "bg-info text-canvas" },
};

export function toneText(tone: PillTone): string {
  return TONE_CLASSES[tone].text;
}
export function toneBg(tone: PillTone): string {
  return TONE_CLASSES[tone].bg;
}
export function toneTint(tone: PillTone): string {
  return TONE_CLASSES[tone].tint;
}
export function toneSolid(tone: PillTone): string {
  return TONE_CLASSES[tone].solid;
}
