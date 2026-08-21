import {
  AA_TEXT_CONTRAST,
  compositeOver,
  contrastRatio,
  formatHexColor,
  mixColors,
  parseHexColor,
  type Rgb,
} from "./contrast";

export interface IdentityInk {
  /** Hue-tinted ink measured to clear the AA text floor against its background. */
  fg: string;
  /** The measured contrast ratio of `fg` over that background. */
  ratio: number;
}

export interface IdentityColors extends IdentityInk {
  /** The composited plate color `fg` was measured against. */
  bg: string;
}

/** Mirrored from `--color-canvas-soft`; the token-contract suite prevents drift. */
export const IDENTITY_SURFACE_TOKEN = "color-canvas-soft";
export const IDENTITY_SURFACE_VALUE = "#1f1e1c";

/** Neutral fallback for an absent or malformed identity color. */
export const IDENTITY_FALLBACK_COLOR = "#8a8f98";

/** Alpha of the identity color in its plate. */
const FILL_ALPHA = 0.2;
/** How far the resting ink is mixed toward the contrast pole. */
const INK_BASE_MIX = 0.28;
/** Step size when the resting ink does not clear the floor. */
const INK_STEP = 0.06;

const WHITE: Rgb = { r: 255, g: 255, b: 255 };
const BLACK: Rgb = { r: 0, g: 0, b: 0 };

function resolve(color: string | undefined, fallback: string): Rgb {
  const parsed = color === undefined ? null : parseHexColor(color);
  return parsed ?? (parseHexColor(fallback) as Rgb);
}

function inkPole(plate: Rgb): Rgb {
  return contrastRatio(WHITE, plate) >= contrastRatio(BLACK, plate) ? WHITE : BLACK;
}

/** Measure the same 8-bit color the browser paints. */
function quantize(color: Rgb): Rgb {
  return parseHexColor(formatHexColor(color)) as Rgb;
}

/** Derives hue-tinted ink that clears the AA text floor. */
export function identityInkOn(color: string | undefined, background: Rgb): IdentityInk {
  const identity = resolve(color, IDENTITY_FALLBACK_COLOR);
  const pole = inkPole(background);

  let ink = quantize(mixColors(identity, pole, INK_BASE_MIX));
  let ratio = contrastRatio(ink, background);
  for (let mix = INK_BASE_MIX + INK_STEP; ratio < AA_TEXT_CONTRAST && mix <= 1; mix += INK_STEP) {
    ink = quantize(mixColors(identity, pole, mix));
    ratio = contrastRatio(ink, background);
  }

  return { fg: formatHexColor(ink), ratio };
}

/** Composites the identity plate a glyph or selected cell is painted on. */
export function identityPlateFor(color?: string, surface?: string): string {
  const identity = resolve(color, IDENTITY_FALLBACK_COLOR);
  const base = resolve(surface, IDENTITY_SURFACE_VALUE);
  return formatHexColor(quantize(compositeOver(identity, FILL_ALPHA, base)));
}

/** The plate plus the ink measured against it — what a glyph paints. */
export function identityColorsFor(color?: string, surface?: string): IdentityColors {
  const bg = identityPlateFor(color, surface);
  const { fg, ratio } = identityInkOn(color, parseHexColor(bg) as Rgb);
  return { bg, fg, ratio };
}
