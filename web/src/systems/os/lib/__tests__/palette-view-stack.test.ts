// Suite: palette view stack mechanics
// Invariant: pushing and popping preserve the exact path at any depth, and the
// breadcrumb keeps the leaf and its parent while collapsing every earlier level
// into one slot.
// Boundary IN: stack frames and breadcrumb labels.
// Boundary OUT: store wiring, view registration, rendered palette behavior, and
// keyboard-selection survival — that helper is `resolveCommandSelection`, owned
// by the `@compozy/ui` Command suite.
import { describe, expect, it } from "vitest";

import {
  activePaletteViewId,
  PALETTE_BREADCRUMB_MAX_SLOTS,
  paletteBreadcrumb,
  popPaletteViewFrame,
  pushPaletteViewFrame,
  type PaletteViewFrame,
} from "../palette-view-stack";

const ROOT: readonly PaletteViewFrame[] = [];

describe("palette view stack (UT-059)", () => {
  it("Should push and pop levels in exact order and report the active view", () => {
    expect(activePaletteViewId(ROOT)).toBeNull();

    const oneDeep = pushPaletteViewFrame(ROOT, "sessions");
    const twoDeep = pushPaletteViewFrame(oneDeep, "sessions");
    const threeDeep = pushPaletteViewFrame(twoDeep, "sessions");

    expect(threeDeep).toHaveLength(3);
    expect(activePaletteViewId(threeDeep)).toBe("sessions");
    expect(popPaletteViewFrame(threeDeep)).toEqual(twoDeep);
    expect(popPaletteViewFrame(popPaletteViewFrame(threeDeep))).toEqual(oneDeep);
    expect(popPaletteViewFrame(popPaletteViewFrame(popPaletteViewFrame(threeDeep)))).toEqual(ROOT);
    expect(oneDeep).toHaveLength(1);
  });

  it("Should leave the root unchanged when popping past it", () => {
    expect(popPaletteViewFrame(ROOT)).toEqual(ROOT);
    expect(activePaletteViewId(popPaletteViewFrame(ROOT))).toBeNull();
  });

  it("Should keep the leaf and its parent and collapse earlier levels into one slot", () => {
    expect(paletteBreadcrumb([])).toEqual({ truncated: false, visible: [] });
    expect(paletteBreadcrumb(["Sessions"])).toEqual({ truncated: false, visible: ["Sessions"] });
    expect(paletteBreadcrumb(["Sessions", "claude"])).toEqual({
      truncated: false,
      visible: ["Sessions", "claude"],
    });

    const deep = paletteBreadcrumb(["Workspaces", "Sessions", "claude"]);
    expect(deep).toEqual({ truncated: true, visible: ["Sessions", "claude"] });

    const deeper = paletteBreadcrumb(["A", "B", "C", "D", "E"]);
    expect(deeper).toEqual({ truncated: true, visible: ["D", "E"] });
    expect(deeper.visible.length + 1).toBeLessThanOrEqual(PALETTE_BREADCRUMB_MAX_SLOTS);
  });
});
