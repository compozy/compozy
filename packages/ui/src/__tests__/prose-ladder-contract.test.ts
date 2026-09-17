import { readFileSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import { AA_TEXT_CONTRAST, contrastRatio, parseHexColor, type Rgb } from "../lib/contrast";

// Token contract for rendered Markdown: headings outrank the prose body they introduce, and links stay legible.

const TOKENS_CSS = readFileSync(join(__dirname, "..", "tokens.css"), "utf8");

// Heading tiers from largest to smallest, ending at the prose body size used by <Markdown>.
const PROSE_LADDER = [
  "text-prose-h1",
  "text-prose-h2",
  "text-prose-h3",
  "text-card-title",
] as const;

// Surfaces a Markdown column renders on: chat thread, cards, and tinted panels.
const PROSE_SURFACES = ["color-canvas", "color-canvas-soft", "color-canvas-tint"] as const;

function readToken(name: string): string {
  const match = TOKENS_CSS.match(new RegExp(`--${name}\\s*:\\s*([^;]+);`));
  if (!match) throw new Error(`token --${name} not found in tokens.css`);
  return match[1].replace(/\s+/g, " ").trim();
}

function readRem(name: string): number {
  const value = readToken(name);
  const match = value.match(/^(\d+(?:\.\d+)?)rem$/);
  if (!match) throw new Error(`expected a rem size for --${name}, got "${value}"`);
  return Number(match[1]);
}

function readHex(name: string): Rgb {
  const parsed = parseHexColor(readToken(name));
  if (!parsed) throw new Error(`expected a hex color for --${name}`);
  return parsed;
}

describe("prose heading ladder token contract", () => {
  for (let index = 0; index < PROSE_LADDER.length - 1; index += 1) {
    const upper = PROSE_LADDER[index];
    const lower = PROSE_LADDER[index + 1];
    it(`Should keep --${upper} larger than --${lower}`, () => {
      expect(readRem(upper)).toBeGreaterThan(readRem(lower));
    });
  }
});

describe("prose link color token contract", () => {
  const link = readHex("color-accent-strong");
  for (const surface of PROSE_SURFACES) {
    it(`Should hold ≥${AA_TEXT_CONTRAST}:1 for --color-accent-strong on --${surface}`, () => {
      const ratio = contrastRatio(link, readHex(surface));
      expect(ratio, `${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(AA_TEXT_CONTRAST);
    });
  }
});
