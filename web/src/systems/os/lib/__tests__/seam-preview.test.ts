// Suite: seam drag preview math
// Invariant: the live seam preview and the committed layout.resize apply the same
// weight-space delta — px clamped by projection minimums, weights floored at the
// daemon's 0.01 adjacent minimum, normalized by the split's axis span.
// Boundary IN: a projected seam + raw pointer delta in pixels.
// Boundary OUT: gesture lifecycle and command transport.
import { describe, expect, it } from "vitest";

import { applySeamPreviewToDesktop, seamWeightDelta } from "../seam-preview";
import type { LayoutDesktop, ProjectedSeam } from "../window-manager-types";

const SEAM: ProjectedSeam = {
  id: "split:root:0",
  splitId: "split:root",
  boundaryIndex: 0,
  orientation: "vertical",
  rect: { x: 496, y: 0, w: 8, h: 700 },
  value: 500,
  minValue: 300,
  maxValue: 700,
  axisSpan: 992,
  leadingWeight: 0.5,
  trailingWeight: 0.5,
  leadingWindowIds: ["window:left"],
  trailingWindowIds: ["window:right"],
};

function desktop(): LayoutDesktop {
  return {
    id: "desktop:one",
    name: "One",
    order: 0,
    purpose: "standard",
    focusOwner: null,
    floating: [],
    groups: [
      {
        id: "group:main",
        frame: { x: 0, y: 0, w: 1, h: 1 },
        root: {
          id: "split:root",
          kind: "split",
          axis: "horizontal",
          weights: [0.5, 0.5],
          children: [
            { id: "leaf:left", kind: "leaf", windowId: "window:left" },
            { id: "leaf:right", kind: "leaf", windowId: "window:right" },
          ],
        },
      },
    ],
  };
}

describe("seamWeightDelta", () => {
  it("Should convert a pixel drag into the axis-span-normalized weight delta", () => {
    expect(seamWeightDelta(SEAM, 99.2)).toBeCloseTo(0.1, 10);
    expect(seamWeightDelta(SEAM, -99.2)).toBeCloseTo(-0.1, 10);
  });

  it("Should clamp the drag to the projection's pixel minimums", () => {
    // minValue caps leftward travel at value-300 = -200px.
    expect(seamWeightDelta(SEAM, -500)).toBeCloseTo(-200 / 992, 10);
    expect(seamWeightDelta(SEAM, 500)).toBeCloseTo(200 / 992, 10);
  });

  it("Should floor both adjacent weights at the daemon minimum", () => {
    const skewed: ProjectedSeam = {
      ...SEAM,
      value: 970,
      minValue: 0,
      maxValue: 992,
      leadingWeight: 0.98,
      trailingWeight: 0.02,
    };
    expect(seamWeightDelta(skewed, 992)).toBeCloseTo(0.01, 10);
    expect(seamWeightDelta(skewed, -992)).toBeCloseTo(-0.97, 10);
  });

  it("Should return zero for a degenerate span or non-finite delta", () => {
    expect(seamWeightDelta({ ...SEAM, axisSpan: 0 }, 100)).toBe(0);
    expect(seamWeightDelta(SEAM, Number.NaN)).toBe(0);
  });
});

describe("applySeamPreviewToDesktop", () => {
  it("Should shift only the two adjacent weights of the previewed split", () => {
    const adjusted = applySeamPreviewToDesktop(desktop(), SEAM, 99.2);
    const root = adjusted.groups[0]?.root;
    if (root?.kind !== "split") throw new Error("expected split root");
    expect(root.weights[0]).toBeCloseTo(0.6, 10);
    expect(root.weights[1]).toBeCloseTo(0.4, 10);
  });

  it("Should return the desktop unchanged for a zero delta", () => {
    const base = desktop();
    expect(applySeamPreviewToDesktop(base, SEAM, 0)).toBe(base);
  });

  it("Should preserve the desktop when the projected split is absent", () => {
    const base = desktop();
    expect(applySeamPreviewToDesktop(base, { ...SEAM, splitId: "split:missing" }, 99.2)).toBe(base);
  });

  it("Should preserve unaffected group identities when one split changes", () => {
    const base = desktop();
    const untouched = {
      id: "group:untouched",
      frame: { x: 0, y: 0, w: 1, h: 1 },
      root: { id: "leaf:untouched", kind: "leaf", windowId: "window:untouched" },
    } as const;
    const withPeer = { ...base, groups: [...base.groups, untouched] };

    const adjusted = applySeamPreviewToDesktop(withPeer, SEAM, 99.2);

    expect(adjusted).not.toBe(withPeer);
    expect(adjusted.groups[0]).not.toBe(withPeer.groups[0]);
    expect(adjusted.groups[1]).toBe(untouched);
  });
});
