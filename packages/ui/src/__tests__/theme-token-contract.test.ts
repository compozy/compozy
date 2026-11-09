import { describe, expect, it } from "vitest";

import {
  AA_NON_TEXT_CONTRAST,
  AA_TEXT_CONTRAST,
  compositeOver,
  contrastRatio,
  parseHexColor,
  parseRgbaColor,
  type Rgb,
} from "../lib/contrast";
import {
  TERMINAL_TOKENS_CSS,
  THEMES,
  TOKENS_CSS,
  TOKENS_LIGHT_CSS,
  TOKENS_RUNTIME_CSS,
  readToken,
  type Theme,
} from "./token-source";

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
 * in both themes: panes, cards, sunken insets, chrome, hover steps, selected rows, and
 * popovers. The reference palette's dimmer greys (#a1a1a1 on white) fail this; the
 * production override does not.
 */
const TEXT_TOKENS = [
  "color-fg-2",
  "color-muted-foreground",
  "color-subtle",
  "color-faint",
] as const;
const TEXT_SURFACES = [
  "color-canvas",
  "color-canvas-soft",
  "color-sunken",
  "color-rail",
  "color-surface-2",
  "color-selected",
  "color-elevated",
] as const;

function readHex(name: string, theme: Theme): Rgb {
  const parsed = parseHexColor(readToken(name, theme));
  if (!parsed) throw new Error(`expected a hex color for --${name} (${theme})`);
  return parsed;
}

/**
 * Chrome-plate ink floor, per theme. A hovered or selected rail / topbar / deck
 * plate steps its label to `fg-2` or `fg`; `muted` is not a chrome-plate ink (in
 * light it falls below AA on the darker plates), so only the plate inks are held
 * to AA there.
 */
const CHROME_PLATE_INKS = ["color-fg", "color-fg-2"] as const;
const CHROME_PLATES = ["color-rail-hover", "color-rail-selected"] as const;

describe.each(THEMES)("chrome plate ink contrast floor (%s theme)", theme => {
  for (const ink of CHROME_PLATE_INKS) {
    it(`Should hold ≥${AA_TEXT_CONTRAST}:1 for --${ink} on every chrome plate`, () => {
      for (const plate of CHROME_PLATES) {
        const ratio = contrastRatio(readHex(ink, theme), readHex(plate, theme));
        expect(ratio, `--${ink} on --${plate} = ${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(
          AA_TEXT_CONTRAST
        );
      }
    });
  }
});

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

/**
 * Non-text indicator floor, per theme (DESIGN.md accessibility floor). A resting
 * dot, ring or mark that alone carries state rides `--color-indicator`, so it must
 * clear 3:1 on every surface it can rest on; hairlines stay decorative and are not
 * held to it. The identity well's glyph and the brand count pill are paired inks:
 * the glyph clears 3:1 on its well, and the count's text clears AA on its fill.
 */
const INDICATOR_SURFACES = [
  "color-rail",
  "color-desk",
  "color-canvas",
  "color-sunken",
  "color-surface-2",
  "color-selected",
] as const;

describe.each(THEMES)("non-text indicator contrast floor (%s theme)", theme => {
  it(`Should hold ≥${AA_NON_TEXT_CONTRAST}:1 for --color-indicator on every resting surface`, () => {
    for (const surface of INDICATOR_SURFACES) {
      const ratio = contrastRatio(readHex("color-indicator", theme), readHex(surface, theme));
      expect(
        ratio,
        `--color-indicator on --${surface} = ${ratio.toFixed(2)}:1`
      ).toBeGreaterThanOrEqual(AA_NON_TEXT_CONTRAST);
    }
  });

  it(`Should hold ≥${AA_NON_TEXT_CONTRAST}:1 for the identity-well glyph on its well`, () => {
    const ratio = contrastRatio(readHex("color-well-ink", theme), readHex("color-well", theme));
    expect(
      ratio,
      `--color-well-ink on --color-well = ${ratio.toFixed(2)}:1`
    ).toBeGreaterThanOrEqual(AA_NON_TEXT_CONTRAST);
  });

  it(`Should hold ≥${AA_TEXT_CONTRAST}:1 for count ink on the brand fill`, () => {
    const ratio = contrastRatio(readHex("color-brand-ink", theme), readHex("color-brand", theme));
    expect(
      ratio,
      `--color-brand-ink on --color-brand = ${ratio.toFixed(2)}:1`
    ).toBeGreaterThanOrEqual(AA_TEXT_CONTRAST);
  });
});

describe("brand, destructive and muted token names", () => {
  const LEGACY_NAME = /^color-(?:accent(?:-.+)?|danger(?:-.+)?|muted)$/;

  it("Should declare no legacy color token in any token file", () => {
    const declared = [
      TOKENS_CSS,
      TOKENS_LIGHT_CSS,
      TOKENS_RUNTIME_CSS,
      TERMINAL_TOKENS_CSS,
    ].flatMap(declaredNames);
    expect(declared.filter(name => LEGACY_NAME.test(name))).toEqual([]);
  });

  it.each<[Theme, string]>([
    ["dark", "#989898"],
    ["light", "#6b6b6b"],
  ])("Should keep the secondary-text gray in %s", (theme, gray) => {
    expect(parseHexColor(readToken("color-muted-foreground", theme))).toEqual(parseHexColor(gray));
  });
});

/**
 * Brand identity v1.2 (UT-006). Volt is the brand fill, brand-as-text rides
 * `brand-strong`, needs-you is its own amber (never an alias of brand), and the
 * destructive-solid fill keeps the ink the brand fill used to carry.
 */
describe("brand identity v1.2 token values", () => {
  it.each<[string, string, string]>([
    ["color-brand", "#2f5bff", "#2448e6"],
    ["color-brand-hover", "#2448e6", "#1f3fcc"],
    ["color-brand-strong", "#6687ff", "#2448e6"],
    ["color-brand-ink", "#ffffff", "#ffffff"],
    ["color-attn", "#f5b14c", "#9a5b00"],
    ["color-destructive-ink", "#17110f", "#ffffff"],
    ["color-brand-volt", "#2f5bff", "#2f5bff"],
    ["color-brand-sky", "#1f87fc", "#1f87fc"],
    ["color-brand-teal", "#1df0d7", "#1df0d7"],
    ["color-brand-cyan", "#3df2e0", "#3df2e0"],
    ["color-brand-glint", "#b5faf6", "#b5faf6"],
    ["color-brand-deep", "#0b31f7", "#0b31f7"],
  ])("Should hold --%s at %s dark and %s light", (name, dark, light) => {
    expect(parseHexColor(readToken(name, "dark"))).toEqual(parseHexColor(dark));
    expect(parseHexColor(readToken(name, "light"))).toEqual(parseHexColor(light));
  });

  it.each<[string, string, string]>([
    ["color-brand-tint", "rgba(47, 91, 255, 0.1)", "rgba(36, 72, 230, 0.08)"],
    ["color-brand-tint-strong", "rgba(47, 91, 255, 0.16)", "rgba(36, 72, 230, 0.14)"],
    ["color-brand-dim", "rgba(47, 91, 255, 0.24)", "rgba(36, 72, 230, 0.22)"],
    ["color-brand-glow", "rgba(47, 91, 255, 0.05)", "rgba(36, 72, 230, 0.05)"],
  ])("Should derive --%s from volt in both themes", (name, dark, light) => {
    expect(readToken(name, "dark")).toBe(dark);
    expect(readToken(name, "light")).toBe(light);
  });

  it("Should give attention its own value, never an alias of brand", () => {
    expect(TOKENS_CSS).not.toMatch(/--color-attn:\s*var\(--color-brand\)/);
    expect(TOKENS_LIGHT_CSS).not.toMatch(/--color-attn:\s*var\(/);
  });

  it("Should keep primary actions inverted, never volt", () => {
    expect(parseHexColor(readToken("color-primary", "dark"))).toEqual(parseHexColor("#f5f5f5"));
    expect(parseHexColor(readToken("color-primary", "light"))).toEqual(parseHexColor("#0b0d12"));
  });

  it("Should declare every brand gradient at 45deg without deep", () => {
    const gradients = Array.from(
      TOKENS_CSS.matchAll(/--(gradient-[a-z-]+)\s*:\s*([^;]+);/g),
      match => [match[1], match[2]] as const
    );
    expect(gradients.map(([name]) => name).sort()).toEqual(["gradient-brand", "gradient-live"]);
    for (const [name, value] of gradients) {
      expect(value, name).toMatch(/^linear-gradient\(45deg,/);
      expect(value.toLowerCase(), name).not.toContain("#0b31f7");
      expect(value, name).not.toContain("brand-deep");
    }
    expect(readToken("gradient-brand")).toBe(
      "linear-gradient(45deg, #1f87fc 0%, #1df0d7 60%, #b5faf6 100%)"
    );
    expect(readToken("gradient-live")).toBe("linear-gradient(45deg, #2f5bff, #3df2e0)");
  });
});

/**
 * Brand text pairs (UT-007, ported from the brand manual's contrast pairs onto
 * product tokens). Each text/background pair clears 4.5:1 — 3:1 where the
 * pairing is large-only — in every theme it is used; a failure names the pair.
 * A translucent background is composited over the surface it sits on.
 */
type ContrastPair = readonly [fg: string, bg: string, min: number, themes: readonly Theme[]];

const BRAND_TEXT_SURFACES = [
  "color-canvas",
  "color-canvas-soft",
  "color-sunken",
  "color-rail",
  "color-surface-2",
  "color-selected",
  "color-elevated",
] as const;

const BRAND_CONTRAST_PAIRS: readonly ContrastPair[] = [
  ["color-fg", "color-rail", AA_TEXT_CONTRAST, THEMES],
  ["color-muted-foreground", "color-rail", AA_TEXT_CONTRAST, THEMES],
  ["color-brand-ink", "color-brand", AA_TEXT_CONTRAST, THEMES],
  ["color-brand-ink", "color-brand-hover", AA_TEXT_CONTRAST, THEMES],
  ["color-destructive-ink", "color-destructive", AA_TEXT_CONTRAST, THEMES],
  ["color-brand-strong", "color-brand-tint", AA_TEXT_CONTRAST, THEMES],
  ["color-attn", "color-attn-tint", AA_TEXT_CONTRAST, THEMES],
  ["color-attn", "color-canvas", AA_TEXT_CONTRAST, THEMES],
  ["color-attn", "color-rail", AA_TEXT_CONTRAST, THEMES],
  ["color-warning", "color-warning-tint", AA_TEXT_CONTRAST, THEMES],
  ["color-destructive", "color-destructive-tint", AA_TEXT_CONTRAST, THEMES],
  ["color-success", "color-success-tint", AA_TEXT_CONTRAST, THEMES],
  ["color-warning", "color-canvas", AA_TEXT_CONTRAST, THEMES],
  // Solid signal plates carry `canvas` ink (pill-variants, tone, avatar).
  ["color-canvas", "color-attn", AA_TEXT_CONTRAST, THEMES],
  ["color-canvas", "color-success", AA_TEXT_CONTRAST, THEMES],
  ["color-canvas", "color-warning", AA_TEXT_CONTRAST, THEMES],
  ["color-canvas", "color-info", AA_TEXT_CONTRAST, THEMES],
  ...BRAND_TEXT_SURFACES.map(
    surface => ["color-brand-strong", surface, AA_TEXT_CONTRAST, THEMES] as const
  ),
  // Signal labels on ink (dark only: cyan is a dark-ground signal).
  ["color-brand-cyan", "color-rail", AA_TEXT_CONTRAST, ["dark"]],
  // Volt itself as text is large-only on ink.
  ["color-brand", "color-rail", AA_NON_TEXT_CONTRAST, ["dark"]],
];

function surfaceRgb(name: string, theme: Theme): Rgb {
  const value = readToken(name, theme);
  const hex = parseHexColor(value);
  if (hex) return hex;
  const rgba = parseRgbaColor(value);
  if (!rgba) throw new Error(`expected a color for --${name} (${theme}), got "${value}"`);
  return compositeOver(rgba.rgb, rgba.alpha, readHex("color-canvas", theme));
}

describe.each(THEMES)("brand text contrast pairs (%s theme)", theme => {
  for (const [fg, bg, min, themes] of BRAND_CONTRAST_PAIRS) {
    if (!themes.includes(theme)) continue;
    it(`Should hold ≥${min}:1 for --${fg} on --${bg}`, () => {
      const ratio = contrastRatio(readHex(fg, theme), surfaceRgb(bg, theme));
      expect(ratio, `--${fg} on --${bg} (${theme}) = ${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(
        min
      );
    });
  }
});

/**
 * Ink on the brand gradients and bright signal plates. Text on a gradient must
 * clear the floor against every stop (the worst stop decides), so the fixed
 * `--color-on-gradient` ink is checked against each literal stop of
 * `--gradient-brand` (4.5:1) and `--gradient-live` (3:1, large text only — its
 * volt stop cannot carry body text in any ink). White is never the gradient ink.
 */
function gradientStops(name: string): Rgb[] {
  return Array.from(readToken(name).matchAll(/#[0-9a-fA-F]{6}\b/g), match => {
    const rgb = parseHexColor(match[0]);
    if (!rgb) throw new Error(`bad stop ${match[0]} in --${name}`);
    return rgb;
  });
}

const GRADIENT_INK_FLOORS: readonly [gradient: string, min: number][] = [
  ["gradient-brand", AA_TEXT_CONTRAST],
  ["gradient-live", AA_NON_TEXT_CONTRAST],
];

// Bright plates that carry `--color-on-gradient` labels; attn and success are
// dark-ground fills only (light themes darken them for text).
const ON_GRADIENT_PLATES: readonly [plate: string, themes: readonly Theme[]][] = [
  ["color-brand-sky", THEMES],
  ["color-brand-teal", THEMES],
  ["color-brand-cyan", THEMES],
  ["color-brand-glint", THEMES],
  ["color-attn", ["dark"]],
  ["color-success", ["dark"]],
];

describe("ink on the brand gradients", () => {
  it.each(GRADIENT_INK_FLOORS)(
    "Should hold --color-on-gradient on every stop of --%s",
    (name, min) => {
      const stops = gradientStops(name);
      expect(stops.length).toBeGreaterThanOrEqual(2);
      for (const stop of stops) {
        const ratio = contrastRatio(readHex("color-on-gradient", "dark"), stop);
        expect(
          ratio,
          `--color-on-gradient on a --${name} stop = ${ratio.toFixed(2)}:1`
        ).toBeGreaterThanOrEqual(min);
      }
    }
  );

  it("Should keep the gradient ink fixed across themes", () => {
    expect(readHex("color-on-gradient", "light")).toEqual(readHex("color-on-gradient", "dark"));
  });

  it("Should fail white on the brand gradient, so white never becomes its ink", () => {
    const white = parseHexColor("#ffffff") as Rgb;
    const worst = Math.min(
      ...gradientStops("gradient-brand").map(stop => contrastRatio(white, stop))
    );
    expect(worst).toBeLessThan(AA_NON_TEXT_CONTRAST);
  });

  for (const theme of THEMES) {
    for (const [plate, themes] of ON_GRADIENT_PLATES) {
      if (!themes.includes(theme)) continue;
      it(`Should hold ≥${AA_TEXT_CONTRAST}:1 for --color-on-gradient on --${plate} (${theme})`, () => {
        const ratio = contrastRatio(readHex("color-on-gradient", theme), readHex(plate, theme));
        expect(
          ratio,
          `--color-on-gradient on --${plate} (${theme}) = ${ratio.toFixed(2)}:1`
        ).toBeGreaterThanOrEqual(AA_TEXT_CONTRAST);
      });
    }
  }
});

/**
 * Light terminal foreground readability. Programs print any ANSI lane as copy —
 * SGR 30 (ANSI 0, "black") included — so on the light well every lane clears
 * AA text contrast. ANSI 8 is the one deliberately dim metadata lane.
 */
const TERMINAL_TEXT_LANES = Array.from({ length: 16 }, (_, lane) => lane).filter(
  lane => lane !== 8
);

describe("light terminal foreground readability", () => {
  it.each(TERMINAL_TEXT_LANES)(
    `Should hold ≥${AA_TEXT_CONTRAST}:1 for ANSI %i on the light well`,
    lane => {
      const ratio = contrastRatio(
        readHex(`terminal-ansi-${lane}`, "light"),
        readHex("terminal-bg", "light")
      );
      expect(
        ratio,
        `--terminal-ansi-${lane} on --terminal-bg (light) = ${ratio.toFixed(2)}:1`
      ).toBeGreaterThanOrEqual(AA_TEXT_CONTRAST);
    }
  );
});
