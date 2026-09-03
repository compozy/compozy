import { afterEach, describe, expect, it } from "vitest";

import { loadTerminalFonts, parseFontFamilies } from "../terminal-fonts";
import { createFakeFontFaceSet, installFakeFontFaceSet } from "./fake-font-face-set";

/**
 * Canonical suite for the terminal font-readiness module.
 *
 * Invariant: the resolved CSS stack becomes loadable specs (generics, quotes,
 * and duplicates never reach the platform); first paint waits for residency
 * only within the budget; `settled` reports `true` exactly when a face that
 * was missing at call time became resident — the one case where drawn glyphs
 * can be stale.
 */

const STACK = '"JetBrains Mono Variable", "Symbols Nerd Font Mono", monospace';

let restoreFonts: (() => void) | null = null;
let mounted: HTMLElement[] = [];

function terminalElement(fontFamily: string = STACK): HTMLElement {
  const element = document.createElement("div");
  element.style.fontFamily = fontFamily;
  element.style.fontSize = "12px";
  document.body.appendChild(element);
  mounted.push(element);
  return element;
}

afterEach(() => {
  restoreFonts?.();
  restoreFonts = null;
  for (const element of mounted.splice(0)) element.remove();
});

describe("parseFontFamilies", () => {
  it("Should strip quotes, drop generic keywords, and dedupe while keeping order", () => {
    const families = parseFontFamilies(
      `"JetBrains Mono Variable", 'JetBrains Mono', Symbols Nerd Font Mono, monospace, SANS-SERIF, ui-monospace, "JetBrains Mono"`
    );
    expect(families).toEqual([
      "JetBrains Mono Variable",
      "JetBrains Mono",
      "Symbols Nerd Font Mono",
    ]);
  });

  it("Should yield nothing for a stack of generics alone", () => {
    expect(parseFontFamilies("monospace, system-ui")).toEqual([]);
  });
});

describe("loadTerminalFonts", () => {
  it("Should resolve immediately and report nothing loaded when the platform has no FontFaceSet", async () => {
    // jsdom ships no document.fonts, which is exactly the environment under test.
    const load = loadTerminalFonts(terminalElement());
    await expect(load.firstPaint).resolves.toBeUndefined();
    await expect(load.settled).resolves.toBe(false);
  });

  it("Should ask the platform for every concrete family with a probe reaching the symbols ranges", async () => {
    const fonts = createFakeFontFaceSet();
    restoreFonts = installFakeFontFaceSet(fonts);

    const load = loadTerminalFonts(terminalElement(), 1000);
    expect(fonts.loadCalls.map(call => call.font)).toEqual([
      '12px "JetBrains Mono Variable"',
      '12px "Symbols Nerd Font Mono"',
    ]);
    // The probe text is what makes a unicode-range face actually download:
    // without a Private Use Area codepoint the platform may skip it entirely.
    expect(fonts.loadCalls.every(call => (call.text ?? "").includes("\u{F0001}"))).toBe(true);

    fonts.settleLoads();
    await expect(load.settled).resolves.toBe(true);
    await expect(load.firstPaint).resolves.toBeUndefined();
  });

  it("Should short-circuit without loading when every face is already resident", async () => {
    const fonts = createFakeFontFaceSet([
      '12px "JetBrains Mono Variable"',
      '12px "Symbols Nerd Font Mono"',
    ]);
    restoreFonts = installFakeFontFaceSet(fonts);

    const load = loadTerminalFonts(terminalElement());
    expect(fonts.loadCalls).toHaveLength(0);
    await expect(load.settled).resolves.toBe(false);
  });

  it("Should release first paint at the budget while loading continues", async () => {
    const fonts = createFakeFontFaceSet();
    restoreFonts = installFakeFontFaceSet(fonts);

    const load = loadTerminalFonts(terminalElement(), 5);
    // Nothing settles the loads; only the budget can release the paint.
    await expect(load.firstPaint).resolves.toBeUndefined();

    fonts.settleLoads();
    await expect(load.settled).resolves.toBe(true);
  });

  it("Should report nothing loaded when every load fails", async () => {
    const fonts = createFakeFontFaceSet();
    restoreFonts = installFakeFontFaceSet(fonts);

    const load = loadTerminalFonts(terminalElement(), 1000);
    fonts.failLoads();
    // A failed load changed no pixels: the fallback that painted is still what
    // is on screen, so no atlas clear is owed.
    await expect(load.settled).resolves.toBe(false);
    await expect(load.firstPaint).resolves.toBeUndefined();
  });

  it("Should resolve immediately when the stack holds only generic families", async () => {
    const fonts = createFakeFontFaceSet();
    restoreFonts = installFakeFontFaceSet(fonts);

    const load = loadTerminalFonts(terminalElement("monospace"));
    expect(fonts.loadCalls).toHaveLength(0);
    await expect(load.settled).resolves.toBe(false);
  });
});
