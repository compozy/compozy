/**
 * Logo tile hints for the public marketplace, ported from the approved draft's snapshot script
 * (`.compozy/tasks/site-redesign/analysis/marketplace-draft-gen.py`):
 *
 * - `bleed`: an opaque raster logo fills its tile instead of sitting at two thirds of it.
 * - `light`: a dark logo on transparency needs the light tile to stay legible.
 *
 * SVGs never bleed; they are light when they declare no fill or only dark fills. One addition to
 * the draft's rule: `currentColor` resolves to the root `color` attribute when there is one, and
 * single-quoted attributes count like double-quoted ones. Rasters are read
 * as RGBA at 64×64: under 8% transparent pixels means bleed, otherwise a mean visible luminance
 * under 105 means light.
 */
import sharp from "sharp";

export interface IconHints {
  bleed: boolean;
  light: boolean;
}

const SAMPLE = 64;
const VISIBLE_ALPHA = 40;
const BLEED_CLEAR_RATIO = 0.08;
const DARK_LUMINANCE = 105;

function luminance(r: number, g: number, b: number): number {
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function isDarkFill(fill: string): boolean {
  const value = fill.trim().toLowerCase();
  if (value === "black" || value === "currentcolor") return true;
  const match = /^#([0-9a-f]{6}|[0-9a-f]{3})$/.exec(value);
  if (!match) return false;
  const hex = match[1].length === 6 ? match[1] : [...match[1]].map(c => c + c).join("");
  const [r, g, b] = [0, 2, 4].map(index => Number.parseInt(hex.slice(index, index + 2), 16));
  return luminance(r, g, b) < DARK_LUMINANCE;
}

export function svgHints(svg: string): IconHints {
  /** `currentColor` follows the root `color` when the SVG declares one (an `<img>` never inherits it). */
  const rootColor = /<svg\b[^>]*\scolor=(["'])([^"']+)\1/i.exec(svg)?.[2];
  const declared = [
    ...[...svg.matchAll(/fill=(["'])([^"']+)\1/g)].map(match => match[2]),
    ...[...svg.matchAll(/fill:\s*([^;"]+)/g)].map(match => match[1]),
  ];
  const fills: string[] = [];
  for (const fill of declared) {
    if (fill === "none") continue;
    fills.push(rootColor && fill.trim().toLowerCase() === "currentcolor" ? rootColor : fill);
  }
  return { bleed: false, light: fills.every(isDarkFill) };
}

export async function rasterHints(bytes: Buffer): Promise<IconHints> {
  const pixels = await sharp(bytes)
    .ensureAlpha()
    .resize(SAMPLE, SAMPLE, { fit: "fill", kernel: "cubic" })
    .raw()
    .toBuffer();
  let visible = 0;
  let total = 0;
  for (let offset = 0; offset < pixels.length; offset += 4) {
    if (pixels[offset + 3] <= VISIBLE_ALPHA) continue;
    visible += 1;
    total += luminance(pixels[offset], pixels[offset + 1], pixels[offset + 2]);
  }
  const clear = 1 - visible / (SAMPLE * SAMPLE);
  const meanLuminance = total / Math.max(1, visible);
  return {
    bleed: clear < BLEED_CLEAR_RATIO,
    light: clear >= BLEED_CLEAR_RATIO && meanLuminance < DARK_LUMINANCE,
  };
}

export function iconHints(bytes: Buffer, mediaType: string): Promise<IconHints> | IconHints {
  return mediaType === "image/svg+xml" ? svgHints(bytes.toString("utf8")) : rasterHints(bytes);
}
