// Suite: setup panel body sizing
// Invariant: the animated body never claims more height than the viewport leaves after the
// panel chrome, so the footer — the only way forward — stays inside the panel.
// Boundary IN: the pure height projection behind useSetupBodyHeight.
// Boundary OUT: ResizeObserver wiring and the rendered panel.
import { describe, expect, it } from "vitest";

import { resolveSetupBodyHeight } from "../use-setup-body-height";

// head 52 + step strip 44 + footer 58 = 154, plus the scrim's 2×24 gutter.
const CHROME_AND_GUTTER = 202;

describe("resolveSetupBodyHeight", () => {
  it("Should take the pane's own height when the viewport has room for it", () => {
    expect(resolveSetupBodyHeight(440, 900)).toBe(440);
  });

  it("Should never exceed what is left after the panel chrome", () => {
    const viewport = 560;

    expect(resolveSetupBodyHeight(720, viewport)).toBe(viewport - CHROME_AND_GUTTER);
  });

  it("Should keep the footer reachable on a viewport shorter than the old 280px floor", () => {
    const viewport = 420;
    const height = resolveSetupBodyHeight(600, viewport);

    expect(height).toBe(218);
    expect((height ?? 0) + CHROME_AND_GUTTER).toBeLessThanOrEqual(viewport);
  });

  it("Should hand sizing back to flex when the chrome alone fills the viewport", () => {
    expect(resolveSetupBodyHeight(600, 202)).toBeNull();
    expect(resolveSetupBodyHeight(600, 120)).toBeNull();
  });

  it("Should hand sizing back to flex before the pane has been laid out", () => {
    expect(resolveSetupBodyHeight(0, 900)).toBeNull();
  });
});
