import { describe, expect, it } from "vitest";

import {
  PROSE_LINK,
  PROSE_TYPE,
  PROSE_TYPE_COMPACT,
} from "../components/custom/markdown-prose-constants";
import {
  AA_NON_TEXT_CONTRAST,
  AA_TEXT_CONTRAST,
  contrastRatio,
  parseHexColor,
  type Rgb,
} from "../lib/contrast";
import { readToken } from "./token-source";

// Token contract for rendered Markdown, read from the same constants the prose components render with.

// H1–H3 must outrank the body; H4 matches it and separates by weight; H5–H6 are a smaller label tier.
const HEADINGS_ABOVE_BODY = ["h1", "h2", "h3"] as const;

// Surfaces a Markdown column renders on: chat thread, cards, tinted panels, and elevated popovers.
const PROSE_SURFACES = [
  "color-canvas",
  "color-canvas-soft",
  "color-canvas-tint",
  "color-elevated",
] as const;

function utilityToken(utility: string, prefix: string): string {
  if (!utility.startsWith(`${prefix}-`))
    throw new Error(`expected a ${prefix}-* utility, got "${utility}"`);
  return utility.slice(prefix.length + 1);
}

function readRem(variantUtility: string): number {
  // Variant prefixes such as `group-data-[compact=true]/md:` do not change the token a utility reads.
  const utility = variantUtility.slice(variantUtility.lastIndexOf(":") + 1);
  const value = readToken(`text-${utilityToken(utility, "text")}`);
  const match = value.match(/^(\d+(?:\.\d+)?)rem$/);
  if (!match) throw new Error(`expected a rem size for ${utility}, got "${value}"`);
  return Number(match[1]);
}

function readHex(name: string): Rgb {
  const parsed = parseHexColor(readToken(name));
  if (!parsed) throw new Error(`expected a hex color for --${name}`);
  return parsed;
}

describe("prose heading ladder token contract", () => {
  const body = readRem(PROSE_TYPE.body);

  for (const level of HEADINGS_ABOVE_BODY) {
    it(`Should render ${level} larger than the prose body`, () => {
      expect(readRem(PROSE_TYPE[level])).toBeGreaterThan(body);
    });
  }

  it("Should keep h1 > h2 > h3 strictly descending", () => {
    expect(readRem(PROSE_TYPE.h1)).toBeGreaterThan(readRem(PROSE_TYPE.h2));
    expect(readRem(PROSE_TYPE.h2)).toBeGreaterThan(readRem(PROSE_TYPE.h3));
  });

  it("Should never render h4 smaller than the prose body", () => {
    expect(readRem(PROSE_TYPE.h4)).toBeGreaterThanOrEqual(body);
  });
});

describe("compact prose heading tier token contract", () => {
  const levels = ["h1", "h2", "h3", "h4"] as const;

  it("Should never grow as the heading level deepens", () => {
    for (let index = 0; index < levels.length - 1; index += 1) {
      expect(readRem(PROSE_TYPE_COMPACT[levels[index]])).toBeGreaterThanOrEqual(
        readRem(PROSE_TYPE_COMPACT[levels[index + 1]])
      );
    }
  });

  for (const level of levels) {
    it(`Should keep compact ${level} no larger than the reading ${level}`, () => {
      expect(readRem(PROSE_TYPE_COMPACT[level])).toBeLessThanOrEqual(readRem(PROSE_TYPE[level]));
    });
  }
});

describe("prose link color token contract", () => {
  const text = readHex(`color-${utilityToken(PROSE_LINK.text, "text")}`);
  const underline = readHex(`color-${utilityToken(PROSE_LINK.underline, "decoration")}`);

  for (const surface of PROSE_SURFACES) {
    it(`Should hold ≥${AA_TEXT_CONTRAST}:1 for link text on --${surface}`, () => {
      const ratio = contrastRatio(text, readHex(surface));
      expect(ratio, `${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(AA_TEXT_CONTRAST);
    });

    it(`Should hold ≥${AA_NON_TEXT_CONTRAST}:1 for the link underline on --${surface}`, () => {
      const ratio = contrastRatio(underline, readHex(surface));
      expect(ratio, `${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(AA_NON_TEXT_CONTRAST);
    });
  }
});
