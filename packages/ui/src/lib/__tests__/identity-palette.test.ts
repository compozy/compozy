// Suite: identity color derivation
// Invariant: a user-chosen identity color always yields a foreground measured to clear the
// WCAG AA text floor against the plate it is painted on, for any input, on any surface in
// the ramp — and the surface constant the math is anchored to never drifts from tokens.css.
// Boundary IN: identityColorsFor and the token constant it declares.
// Boundary OUT: how a consumer applies the returned colors (component suites own that).

import { readFileSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import { AA_TEXT_CONTRAST, contrastRatio, parseHexColor } from "../contrast";
import {
  IDENTITY_FALLBACK_COLOR,
  IDENTITY_SURFACE_TOKEN,
  IDENTITY_SURFACE_VALUE,
  identityColorsFor,
  identityInkOn,
} from "../identity-palette";

const TOKENS_CSS = readFileSync(join(__dirname, "..", "..", "tokens.css"), "utf8");

function readToken(name: string): string {
  const match = TOKENS_CSS.match(new RegExp(`--${name}\\s*:\\s*([^;]+);`));
  if (!match) throw new Error(`token --${name} not found in tokens.css`);
  return match[1].trim();
}

// Every surface an identity plate can land on.
const SURFACE_TOKENS = [
  "color-canvas",
  "color-canvas-soft",
  "color-canvas-tint",
  "color-elevated",
] as const;

// The shipped suggested palette, plus the hostile edges and every signal hex —
// a user may type any of them, and each still has to be readable.
const COLORS = [
  "#8a8f98",
  "#4ea7fc",
  "#2aa8a8",
  "#4cb782",
  "#c26ad6",
  "#e56aa2",
  "#e8b04a",
  "#b58e5f",
  "#ffffff",
  "#000000",
  "#5fbf85",
  "#d6a647",
  "#e0635a",
  "#8e8eb5",
  "#e8572a",
];

describe("identityColorsFor", () => {
  it("Should keep the declared surface constant in step with tokens.css", () => {
    expect(readToken(IDENTITY_SURFACE_TOKEN)).toBe(IDENTITY_SURFACE_VALUE);
  });

  it("Should clear the AA text floor for every identity color on every surface", () => {
    for (const surface of SURFACE_TOKENS) {
      const surfaceValue = readToken(surface);
      for (const color of COLORS) {
        const { bg, fg, ratio } = identityColorsFor(color, surfaceValue);
        const plate = parseHexColor(bg);
        const ink = parseHexColor(fg);
        expect(plate, `${color} on --${surface} produced an unparseable plate`).not.toBeNull();
        expect(ink, `${color} on --${surface} produced an unparseable ink`).not.toBeNull();
        // The reported ratio must be the real one, not a claim.
        const measured = contrastRatio(ink!, plate!);
        expect(measured).toBeCloseTo(ratio, 5);
        expect(ratio, `${color} on --${surface} = ${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(
          AA_TEXT_CONTRAST
        );
      }
    }
  });

  it("Should clear the AA text floor for ink painted directly on a bare surface", () => {
    // The picker grid tints every cell on the panel surface, not on an identity
    // plate, so that background has to be measured on its own terms.
    for (const surface of SURFACE_TOKENS) {
      const background = parseHexColor(readToken(surface))!;
      for (const color of COLORS) {
        const { fg, ratio } = identityInkOn(color, background);
        const measured = contrastRatio(parseHexColor(fg)!, background);
        expect(measured).toBeCloseTo(ratio, 5);
        expect(
          ratio,
          `${color} ink on bare --${surface} = ${ratio.toFixed(2)}:1`
        ).toBeGreaterThanOrEqual(AA_TEXT_CONTRAST);
      }
    }
  });

  it("Should tint the ink toward the identity hue rather than returning flat white", () => {
    const violet = identityColorsFor("#c26ad6");
    expect(violet.fg).not.toBe("#ffffff");
    const ink = parseHexColor(violet.fg)!;
    // Violet keeps a red/blue bias over green once tinted.
    expect(ink.r).toBeGreaterThan(ink.g);
    expect(ink.b).toBeGreaterThan(ink.g);
  });

  it("Should fall back to the neutral identity when the color is absent or malformed", () => {
    const fallback = identityColorsFor(IDENTITY_FALLBACK_COLOR);
    expect(identityColorsFor(undefined)).toEqual(fallback);
    expect(identityColorsFor("not-a-color")).toEqual(fallback);
    expect(identityColorsFor("#12ZZ")).toEqual(fallback);
  });

  it("Should accept shorthand hex and the hashless form", () => {
    const canonical = identityColorsFor("#44bbee");
    expect(identityColorsFor("#4be")).toEqual(canonical);
    expect(identityColorsFor("44bbee")).toEqual(canonical);
  });
});
