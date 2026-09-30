import { describe, expect, it } from "vitest";

import { AA_TEXT_CONTRAST, contrastRatio, parseHexColor, type Rgb } from "../lib/contrast";
import { THEMES, TOKENS_CSS, TOKENS_LIGHT_CSS, readToken, type Theme } from "./token-source";

/**
 * Theme-switch reach contract.
 *
 * The light theme re-declares tokens under `[data-theme="light"]`, which only
 * works for utilities that read the variable at runtime. Tailwind v4 inlines the
 * literal value of some `@theme` namespaces into the generated utility (it parses
 * shadows to thread `--tw-shadow-color` through them), so a light override of such
 * a name never reaches `shadow-*` classes. Those tokens are `@theme` adapters over
 * `--theme-shadow-*` literals, and the light theme overrides the literals instead.
 */

// @theme namespaces whose values Tailwind copies into utilities instead of var().
const INLINED_NAMESPACES = /^(shadow|inset-shadow|drop-shadow|text-shadow)-/;

function declaredNames(css: string): string[] {
  return Array.from(css.matchAll(/--([a-zA-Z0-9-]+)\s*:/g), match => match[1]);
}

function themeBlock(css: string): string {
  const match = css.match(/@theme\s*\{([\s\S]*?)\n\}/);
  if (!match) throw new Error("no @theme block in tokens.css");
  return match[1];
}

const lightNames = declaredNames(TOKENS_LIGHT_CSS);
const themeAdapters = Array.from(
  themeBlock(TOKENS_CSS).matchAll(/--([a-zA-Z0-9-]+)\s*:\s*var\(--(theme-[a-zA-Z0-9-]+)\)\s*;/g),
  match => ({ name: match[1], target: match[2] })
);

describe("theme-switch reach contract", () => {
  it("Should never re-declare a Tailwind-inlined @theme name in the light theme", () => {
    expect(lightNames.filter(name => INLINED_NAMESPACES.test(name))).toEqual([]);
  });

  it("Should back every light --theme-* override with an @theme adapter", () => {
    const adapted = new Set(themeAdapters.map(adapter => adapter.target));
    const orphans = lightNames.filter(name => name.startsWith("theme-") && !adapted.has(name));
    expect(orphans).toEqual([]);
  });

  it("Should give every --theme-* adapter a light value", () => {
    const light = new Set(lightNames);
    const missing = themeAdapters.filter(adapter => !light.has(adapter.target));
    expect(missing).toEqual([]);
  });
});

/**
 * Readable-grey floor, per theme. Secondary copy rides `muted`, `subtle`, `faint`, and
 * `fg-2`, so each must clear WCAG AA text contrast on every surface that carries text
 * in both themes: panes, cards, sunken insets, chrome, and hover steps. The reference
 * palette's dimmer greys (#a1a1a1 on white) fail this; the production override does not.
 */
const TEXT_TOKENS = ["color-fg-2", "color-muted", "color-subtle", "color-faint"] as const;
const TEXT_SURFACES = [
  "color-canvas",
  "color-canvas-soft",
  "color-sunken",
  "color-rail",
  "color-surface-2",
] as const;

function readHex(name: string, theme: Theme): Rgb {
  const parsed = parseHexColor(readToken(name, theme));
  if (!parsed) throw new Error(`expected a hex color for --${name} (${theme})`);
  return parsed;
}

describe.each(THEMES)("text ladder contrast floor (%s theme)", theme => {
  for (const text of TEXT_TOKENS) {
    it(`Should hold ≥${AA_TEXT_CONTRAST}:1 for --${text} on every text surface`, () => {
      for (const surface of TEXT_SURFACES) {
        const ratio = contrastRatio(readHex(text, theme), readHex(surface, theme));
        expect(ratio, `--${text} on --${surface} = ${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(
          AA_TEXT_CONTRAST
        );
      }
    });
  }
});
