/**
 * Font readiness for the emulator.
 *
 * The emulator rasterizes glyphs into a canvas atlas, and canvas text neither
 * triggers a lazy `@font-face` download nor repaints when one lands — a glyph
 * drawn before its face arrived would stay tofu forever. This module asks the
 * document to load every family the container resolved (with a probe that
 * reaches the unicode-range symbols face), bounded so a slow font never blocks
 * the first frame, and reports whether a face arrived late so the caller can
 * clear the atlas exactly once.
 */

import { readTerminalMetrics } from "./terminal-metrics";

export interface TerminalFontLoad {
  /** Resolves when the stack is resident or the budget elapsed. Never rejects. */
  firstPaint: Promise<void>;
  /**
   * Resolves once loading settles: `true` only when a face that was missing at
   * call time became resident, i.e. glyphs already drawn may be stale.
   */
  settled: Promise<boolean>;
}

/**
 * One Latin cell plus a Nerd Font icon in each Private Use Area plane —
 * `FontFaceSet.load` uses the text to pick which unicode-range faces actually
 * need fetching, and probing both planes covers a face split across them.
 */
const FONT_PROBE_TEXT = "W\uE0A0\uE0B0\uE5FA\u{F0001}";

/** How long the first frame may wait for type before painting with fallback. */
export const TERMINAL_FONT_BUDGET_MS = 1500;

/** CSS generic family keywords — resolved by the browser, never loadable. */
const GENERIC_FAMILIES = new Set([
  "cursive",
  "emoji",
  "fangsong",
  "fantasy",
  "math",
  "monospace",
  "sans-serif",
  "serif",
  "system-ui",
  "ui-monospace",
  "ui-rounded",
  "ui-sans-serif",
  "ui-serif",
]);

/** Splits a computed `font-family` list into loadable, quoted-clean names. */
export function parseFontFamilies(fontFamily: string): string[] {
  const families: string[] = [];
  const seen = new Set<string>();
  for (const entry of fontFamily.split(",")) {
    const name = entry
      .trim()
      .replace(/^["']|["']$/g, "")
      .trim();
    if (name === "" || GENERIC_FAMILIES.has(name.toLowerCase())) continue;
    if (seen.has(name)) continue;
    seen.add(name);
    families.push(name);
  }
  return families;
}

const RESOLVED: TerminalFontLoad = {
  firstPaint: Promise.resolve(),
  settled: Promise.resolve(false),
};

/**
 * Loads the font stack the element resolved and reports how it landed.
 *
 * Environments without a `FontFaceSet` (tests, ancient engines) resolve
 * immediately with nothing loaded: the emulator then simply paints with
 * whatever the platform gives it, which is exactly what happened before.
 */
export function loadTerminalFonts(
  element: HTMLElement,
  budgetMs: number = TERMINAL_FONT_BUDGET_MS
): TerminalFontLoad {
  const fonts = element.ownerDocument?.fonts;
  if (!fonts || typeof fonts.load !== "function" || typeof fonts.check !== "function") {
    return RESOLVED;
  }
  const metrics = readTerminalMetrics(element);
  const specs = parseFontFamilies(metrics.fontFamily).map(
    family => `${metrics.fontSize}px "${family}"`
  );
  if (specs.length === 0) return RESOLVED;

  const resident = specs.map(spec => checkQuietly(fonts, spec));
  if (resident.every(Boolean)) return RESOLVED;

  const settled = Promise.allSettled(specs.map(spec => fonts.load(spec, FONT_PROBE_TEXT))).then(
    () =>
      // Only a face that flipped from missing to resident can have left stale
      // glyphs behind; a load that failed outright changed nothing on screen.
      specs.some((spec, index) => !resident[index] && checkQuietly(fonts, spec))
  );

  const firstPaint = new Promise<void>(resolve => {
    const view = element.ownerDocument?.defaultView;
    const timer = view?.setTimeout(resolve, budgetMs);
    void settled.then(() => {
      if (timer !== undefined) view?.clearTimeout(timer);
      resolve();
    });
  });

  return { firstPaint, settled };
}

/** `FontFaceSet.check` throws on unparsable specs; an unknown face is simply not resident. */
function checkQuietly(fonts: FontFaceSet, spec: string): boolean {
  try {
    return fonts.check(spec, FONT_PROBE_TEXT);
  } catch {
    return false;
  }
}
