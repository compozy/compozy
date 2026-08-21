export interface Rgb {
  r: number;
  g: number;
  b: number;
}

const HEX_PATTERN = /^#?([0-9a-f]{3}|[0-9a-f]{6})$/i;

/** Parses `#rgb`, `#rrggbb`, or the same without the hash. Returns null when malformed. */
export function parseHexColor(value: string): Rgb | null {
  const match = value.trim().match(HEX_PATTERN);
  if (!match) return null;
  const digits =
    match[1].length === 3
      ? match[1]
          .split("")
          .map(character => character + character)
          .join("")
      : match[1];
  const int = Number.parseInt(digits, 16);
  return { r: (int >> 16) & 0xff, g: (int >> 8) & 0xff, b: int & 0xff };
}

/** Serializes back to the canonical lowercase `#rrggbb` form. */
export function formatHexColor({ r, g, b }: Rgb): string {
  const channel = (value: number) =>
    Math.round(Math.min(255, Math.max(0, value)))
      .toString(16)
      .padStart(2, "0");
  return `#${channel(r)}${channel(g)}${channel(b)}`;
}

/** Parses an `rgb()` / `rgba()` declaration. Returns null when malformed. */
export function parseRgbaColor(value: string): { rgb: Rgb; alpha: number } | null {
  const match = value
    .trim()
    .match(/^rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*([\d.]+)\s*)?\)$/);
  if (!match) return null;
  return {
    rgb: { r: Number(match[1]), g: Number(match[2]), b: Number(match[3]) },
    alpha: match[4] === undefined ? 1 : Number(match[4]),
  };
}

/** Alpha-composites `fg` over `bg`, yielding the color a viewer actually sees. */
export function compositeOver(fg: Rgb, alpha: number, bg: Rgb): Rgb {
  const clamped = Math.min(1, Math.max(0, alpha));
  return {
    r: clamped * fg.r + (1 - clamped) * bg.r,
    g: clamped * fg.g + (1 - clamped) * bg.g,
    b: clamped * fg.b + (1 - clamped) * bg.b,
  };
}

/** Linear interpolation between two colors in sRGB. `amount` 0 → `from`, 1 → `to`. */
export function mixColors(from: Rgb, to: Rgb, amount: number): Rgb {
  const clamped = Math.min(1, Math.max(0, amount));
  return {
    r: from.r + (to.r - from.r) * clamped,
    g: from.g + (to.g - from.g) * clamped,
    b: from.b + (to.b - from.b) * clamped,
  };
}

export function relativeLuminance({ r, g, b }: Rgb): number {
  const linear = (channel: number): number => {
    const scaled = channel / 255;
    return scaled <= 0.03928 ? scaled / 12.92 : ((scaled + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * linear(r) + 0.7152 * linear(g) + 0.0722 * linear(b);
}

export function contrastRatio(a: Rgb, b: Rgb): number {
  const luminanceA = relativeLuminance(a);
  const luminanceB = relativeLuminance(b);
  const [high, low] =
    luminanceA >= luminanceB ? [luminanceA, luminanceB] : [luminanceB, luminanceA];
  return (high + 0.05) / (low + 0.05);
}

/** WCAG AA floor for normal-size text. */
export const AA_TEXT_CONTRAST = 4.5;
/** WCAG AA floor for large text and non-text indicators (focus rings, borders). */
export const AA_NON_TEXT_CONTRAST = 3;
