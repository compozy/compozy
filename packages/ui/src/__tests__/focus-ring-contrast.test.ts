import { describe, expect, it } from "vitest";

import {
  AA_NON_TEXT_CONTRAST,
  compositeOver,
  contrastRatio,
  parseHexColor,
  parseRgbaColor,
  type Rgb,
} from "../lib/contrast";
import { THEMES, readToken } from "./token-source";

/**
 * Cross-surface computed focus-indicator audit (BUG-20260714), per theme.
 *
 * The shared `:focus-visible` tokens must clear the non-text accessibility floor
 * on every Compozy surface: a ring at least 2 CSS pixels thick with at least 3:1
 * WCAG contrast against both the focused element's own fill and the page
 * background, in both themes. This audit reads the real token bytes from `tokens.css`
 * (dark) and `tokens-light.css` (light), composites
 * the (translucent) ring color over each surface in the ramp, and checks the
 * contrast for every ring-over-surface × adjacent-surface pair. It fails on the
 * pre-fix value (1px, `--color-line-strong` ≈ 1.38:1) and passes on 2px at 50% white
 * (dark) / 50% ink (light).
 *
 * Ownership: this is a design-token contract, so it lives at the `@compozy/ui` token
 * layer — not as a per-component or marketplace-local CSS assertion (which would
 * freeze the wrong ownership layer).
 */

const MIN_RING_PX = 2;
const MIN_NON_TEXT_CONTRAST = AA_NON_TEXT_CONTRAST;

// The exclusive focus tokens (outset + inset). Resting/hairline tokens
// (`--shadow-hairline*` / `--shadow-inset-strong`) intentionally stay on
// `--color-line-*` and are not focus indicators, so they are out of this contract.
const FOCUS_SHADOW_TOKENS = ["shadow-focus-ring", "shadow-focus-inset"] as const;

// Surfaces a focus ring can sit on / sit against — the focused element's fill
// and the page background. Read from the source so the audit tracks the ramp.
const SURFACE_TOKENS = [
  "color-rail",
  "color-canvas",
  "color-canvas-soft",
  "color-sunken",
  "color-surface-2",
  "color-selected",
  "color-elevated",
] as const;

function parseHex(hex: string): Rgb {
  const parsed = parseHexColor(hex);
  if (!parsed) throw new Error(`expected a hex color, got "${hex}"`);
  return parsed;
}

function parseRgba(value: string): { rgb: Rgb; alpha: number } {
  const declaration = value.match(/rgba?\([^)]*\)/);
  if (!declaration) throw new Error(`no rgba() color found in "${value}"`);
  const parsed = parseRgbaColor(declaration[0]);
  if (!parsed) throw new Error(`no rgba() color found in "${value}"`);
  return parsed;
}

function ringWidthPx(shadow: string): number {
  // box-shadow grammar: [inset] <x> <y> <blur> <spread> <color>. The length
  // immediately before the color is the spread — i.e. the ring thickness.
  const match = shadow.match(/(\d+(?:\.\d+)?)px\s+(?:rgba?|hsla?|var|#)/);
  if (!match) throw new Error(`no ring width found in "${shadow}"`);
  return Number(match[1]);
}

describe.each(THEMES)("shared focus indicator token contract (BUG-20260714, %s theme)", theme => {
  const surfaces = SURFACE_TOKENS.map(name => ({ name, rgb: parseHex(readToken(name, theme)) }));

  for (const token of FOCUS_SHADOW_TOKENS) {
    const value = readToken(token, theme);
    const { rgb, alpha } = parseRgba(value);
    const width = ringWidthPx(value);

    it(`Should render --${token} at least ${MIN_RING_PX}px thick`, () => {
      expect(width).toBeGreaterThanOrEqual(MIN_RING_PX);
    });

    for (const over of surfaces) {
      it(`Should hold ≥${MIN_NON_TEXT_CONTRAST}:1 for --${token} composited over --${over.name}`, () => {
        const ring = compositeOver(rgb, alpha, over.rgb);
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

// Chrome plates (hovered / selected rail, topbar and deck items) only ever sit on
// the rail, so a focused plate is audited against itself and the rail — not the
// full pane ramp, which never borders it.
const CHROME_PLATE_TOKENS = ["color-rail-hover", "color-rail-selected"] as const;

describe.each(THEMES)("chrome plate focus indicator contract (%s theme)", theme => {
  const rail = parseHex(readToken("color-rail", theme));

  for (const token of FOCUS_SHADOW_TOKENS) {
    const { rgb, alpha } = parseRgba(readToken(token, theme));

    for (const plateName of CHROME_PLATE_TOKENS) {
      it(`Should hold ≥${MIN_NON_TEXT_CONTRAST}:1 for --${token} on --${plateName} against itself and the rail`, () => {
        const plate = parseHex(readToken(plateName, theme));
        const ring = compositeOver(rgb, alpha, plate);
        for (const [name, against] of [
          [plateName, plate],
          ["color-rail", rail],
        ] as const) {
          const ratio = contrastRatio(ring, against);
          expect(
            ratio,
            `--${token} over --${plateName} vs --${name} = ${ratio.toFixed(2)}:1`
          ).toBeGreaterThanOrEqual(MIN_NON_TEXT_CONTRAST);
        }
      });
    }
  }
});
