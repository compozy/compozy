import { readFileSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

/**
 * Cross-surface computed focus-indicator audit (BUG-20260714).
 *
 * The shared `:focus-visible` tokens must clear the non-text accessibility floor
 * on every AGH surface: a ring at least 2 CSS pixels thick with at least 3:1
 * WCAG contrast against both the focused element's own fill and the page
 * background. This audit reads the real token bytes from `tokens.css`, composites
 * the (translucent) ring color over each surface in the ramp, and checks the
 * contrast for every ring-over-surface × adjacent-surface pair. It fails on the
 * pre-fix value (1px, `--color-line-strong` ≈ 1.38:1) and passes on 2px/50%-white.
 *
 * Ownership: this is a design-token contract, so it lives at the `@agh/ui` token
 * layer — not as a per-component or marketplace-local CSS assertion (which would
 * freeze the wrong ownership layer).
 */

const TOKENS_CSS = readFileSync(join(__dirname, "..", "tokens.css"), "utf8");

const MIN_RING_PX = 2;
const MIN_NON_TEXT_CONTRAST = 3;

// The exclusive focus tokens (outset + inset). Resting/hairline tokens
// (`--shadow-hairline*` / `--shadow-inset-strong`) intentionally stay on
// `--color-line-*` and are not focus indicators, so they are out of this contract.
const FOCUS_SHADOW_TOKENS = ["shadow-focus-ring", "shadow-focus-inset"] as const;

// Surfaces a focus ring can sit on / sit against — the focused element's fill
// and the page background. Read from the source so the audit tracks the ramp.
const SURFACE_TOKENS = [
  "color-canvas",
  "color-canvas-soft",
  "color-canvas-tint",
  "color-elevated",
] as const;

interface Rgb {
  r: number;
  g: number;
  b: number;
}

function readToken(name: string): string {
  const match = TOKENS_CSS.match(new RegExp(`--${name}\\s*:\\s*([^;]+);`));
  if (!match) throw new Error(`token --${name} not found in tokens.css`);
  return match[1].replace(/\s+/g, " ").trim();
}

function parseHex(hex: string): Rgb {
  const match = hex.trim().match(/^#([0-9a-fA-F]{6})$/);
  if (!match) throw new Error(`expected a 6-digit hex color, got "${hex}"`);
  const int = Number.parseInt(match[1], 16);
  return { r: (int >> 16) & 0xff, g: (int >> 8) & 0xff, b: int & 0xff };
}

function parseRgba(value: string): { rgb: Rgb; alpha: number } {
  const match = value.match(/rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*([\d.]+)\s*)?\)/);
  if (!match) throw new Error(`no rgba() color found in "${value}"`);
  return {
    rgb: { r: Number(match[1]), g: Number(match[2]), b: Number(match[3]) },
    alpha: match[4] === undefined ? 1 : Number(match[4]),
  };
}

function ringWidthPx(shadow: string): number {
  // box-shadow grammar: [inset] <x> <y> <blur> <spread> <color>. The length
  // immediately before the color is the spread — i.e. the ring thickness.
  const match = shadow.match(/(\d+(?:\.\d+)?)px\s+(?:rgba?|hsla?|var|#)/);
  if (!match) throw new Error(`no ring width found in "${shadow}"`);
  return Number(match[1]);
}

function composite(fg: Rgb, alpha: number, bg: Rgb): Rgb {
  return {
    r: alpha * fg.r + (1 - alpha) * bg.r,
    g: alpha * fg.g + (1 - alpha) * bg.g,
    b: alpha * fg.b + (1 - alpha) * bg.b,
  };
}

function relativeLuminance({ r, g, b }: Rgb): number {
  const linear = (c: number): number => {
    const s = c / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * linear(r) + 0.7152 * linear(g) + 0.0722 * linear(b);
}

function contrastRatio(a: Rgb, b: Rgb): number {
  const la = relativeLuminance(a);
  const lb = relativeLuminance(b);
  const [hi, lo] = la >= lb ? [la, lb] : [lb, la];
  return (hi + 0.05) / (lo + 0.05);
}

describe("shared focus indicator token contract (BUG-20260714)", () => {
  const surfaces = SURFACE_TOKENS.map(name => ({ name, rgb: parseHex(readToken(name)) }));

  for (const token of FOCUS_SHADOW_TOKENS) {
    const value = readToken(token);
    const { rgb, alpha } = parseRgba(value);
    const width = ringWidthPx(value);

    it(`Should render --${token} at least ${MIN_RING_PX}px thick`, () => {
      expect(width).toBeGreaterThanOrEqual(MIN_RING_PX);
    });

    for (const over of surfaces) {
      it(`Should hold ≥${MIN_NON_TEXT_CONTRAST}:1 for --${token} composited over --${over.name}`, () => {
        const ring = composite(rgb, alpha, over.rgb);
        for (const against of surfaces) {
          const ratio = contrastRatio(ring, against.rgb);
          expect(
            ratio,
            `--${token} over --${over.name} vs --${against.name} = ${ratio.toFixed(2)}:1`
          ).toBeGreaterThanOrEqual(MIN_NON_TEXT_CONTRAST);
        }
      });
    }
  }
});
