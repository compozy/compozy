// Suite: seam drag preview math
// Invariant: the live seam preview and the committed layout.resize apply the same
// weight-space delta — px clamped by projection minimums, weights floored at the
// daemon's 0.01 adjacent minimum, normalized by the split's axis span — and every frame shows
// a shift at which the split still lays out side by side.
// Boundary IN: a projected seam + raw pointer delta in pixels.
// Boundary OUT: seam drag lifecycle and command transport.
import { describe, expect, it } from "vitest";

import { applySeamPreviewToTab, seamWeightDelta } from "../seam-preview";
import { admitSeamShift, projectTab } from "../tab-projection";
import type { LayoutTab, ProjectedSeam } from "../window-manager-types";

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
  trailingWindowIds: ["window:top", "window:bottom"],
};

const UNTOUCHED_BRANCH = {
  id: "split:inner",
  kind: "split",
  axis: "vertical",
  weights: [0.5, 0.5],
  children: [
    { id: "leaf:top", kind: "leaf", windowId: "window:top" },
    { id: "leaf:bottom", kind: "leaf", windowId: "window:bottom" },
  ],
} as const;

function tab(): LayoutTab {
  return {
    id: "tab:main",
    pinned: false,
    root: {
      id: "split:root",
      kind: "split",
      axis: "horizontal",
      weights: [0.5, 0.5],
      children: [{ id: "leaf:left", kind: "leaf", windowId: "window:left" }, UNTOUCHED_BRANCH],
    },
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

  it("Should stop above a collapsed branch's switcher, at its member's minimum (US-009.EC-1)", () => {
    // 500×600, a 300×200 pair on top (collapsed: it cannot sit side by side) over one window.
    const projection = projectTab({
      tab: {
        id: "tab:stack",
        pinned: false,
        root: {
          id: "split:outer",
          kind: "split",
          axis: "vertical",
          weights: [1, 1],
          children: [
            {
              id: "split:top",
              kind: "split",
              axis: "horizontal",
              weights: [1, 1],
              children: [
                { id: "leaf:a", kind: "leaf", windowId: "window:a" },
                { id: "leaf:b", kind: "leaf", windowId: "window:b" },
              ],
            },
            { id: "leaf:c", kind: "leaf", windowId: "window:c" },
          ],
        },
      },
      workArea: { x: 0, y: 0, w: 500, h: 600 },
      gaps: { inner: 0, top: 0, right: 0, bottom: 0, left: 0 },
      minimums: {
        "window:a": { width: 300, height: 200 },
        "window:b": { width: 300, height: 200 },
        "window:c": { width: 300, height: 200 },
      },
    });
    const seam = projection.seams.find(entry => entry.id === "split:outer:0");
    if (seam === undefined) throw new Error("expected the outer seam");

    // Dragging the seam 200px up stops at 236px: 200px of member under the 36px switcher.
    expect(seamWeightDelta(seam, -200)).toBeCloseTo(-64 / 600);
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

describe("applySeamPreviewToTab", () => {
  it("Should shift only the two adjacent weights of the previewed split", () => {
    const adjusted = applySeamPreviewToTab(tab(), SEAM, 99.2);
    const root = adjusted.root;
    if (root.kind !== "split") throw new Error("expected split root");
    expect(root.weights[0]).toBeCloseTo(0.6, 10);
    expect(root.weights[1]).toBeCloseTo(0.4, 10);
  });

  it("Should return the tab unchanged for a zero delta", () => {
    const base = tab();
    expect(applySeamPreviewToTab(base, SEAM, 0)).toBe(base);
  });

  it("Should preserve the tab when the projected split is absent", () => {
    const base = tab();
    expect(applySeamPreviewToTab(base, { ...SEAM, splitId: "split:missing" }, 99.2)).toBe(base);
  });

  it("Should keep a horizontal seam over a collapsed branch laid out when an over-limit preview stops at the switcher-inclusive minimum (F8)", () => {
    // 500×600: a 300×200 pair on top (collapsed: it cannot sit side by side) over one window,
    // stored weights normalized as the daemon keeps them.
    const stacked: LayoutTab = {
      id: "tab:stack",
      pinned: false,
      root: {
        id: "split:outer",
        kind: "split",
        axis: "vertical",
        weights: [0.5, 0.5],
        children: [
          {
            id: "split:top",
            kind: "split",
            axis: "horizontal",
            weights: [0.5, 0.5],
            children: [
              { id: "leaf:a", kind: "leaf", windowId: "window:a" },
              { id: "leaf:b", kind: "leaf", windowId: "window:b" },
            ],
          },
          { id: "leaf:c", kind: "leaf", windowId: "window:c" },
        ],
      },
    };
    const project = (tab: LayoutTab) =>
      projectTab({
        tab,
        workArea: { x: 0, y: 0, w: 500, h: 600 },
        gaps: { inner: 0, top: 0, right: 0, bottom: 0, left: 0 },
        minimums: {
          "window:a": { width: 300, height: 200 },
          "window:b": { width: 300, height: 200 },
          "window:c": { width: 300, height: 200 },
        },
      });
    const seam = project(stacked).seams.find(entry => entry.id === "split:outer:0");
    if (seam === undefined) throw new Error("expected the outer seam");

    // The pointer goes 200px up, 136px past where the member under the switcher reaches 200px.
    const previewed = project(applySeamPreviewToTab(stacked, seam, -200));

    // The outer split is still laid out, its seam at the minimum: 36px switcher + 200px member.
    expect(previewed.collapsed.map(entry => entry.splitId)).toEqual(["split:top"]);
    expect(previewed.seams.find(entry => entry.id === "split:outer:0")?.value).toBe(236);
    expect(previewed.windows.find(entry => entry.windowId === "window:a")?.rect).toEqual({
      x: 0,
      y: 36,
      w: 500,
      h: 200,
    });
  });

  it("Should lay out side by side at every drag the seam admits when its member shares land on half pixels (F8)", () => {
    // The QA geometry: a 290×907 column, an even split over an odd span (453.5px per member),
    // a collapsed pair under its 36px switcher above or below one window, 220px floors.
    const pair = {
      id: "split:pair",
      kind: "split",
      axis: "horizontal",
      weights: [0.5, 0.5],
      children: [
        { id: "leaf:tasks", kind: "leaf", windowId: "window:tasks" },
        { id: "leaf:loops", kind: "leaf", windowId: "window:loops" },
      ],
    } as const;
    const agents = { id: "leaf:agents", kind: "leaf", windowId: "window:agents" } as const;
    const project = (tab: LayoutTab) =>
      projectTab({
        tab,
        workArea: { x: 0, y: 0, w: 290, h: 907 },
        gaps: { inner: 0, top: 0, right: 0, bottom: 0, left: 0 },
        minimums: {
          "window:tasks": { width: 220, height: 220 },
          "window:loops": { width: 220, height: 220 },
          "window:agents": { width: 220, height: 220 },
        },
      });
    for (const children of [
      [pair, agents],
      [agents, pair],
    ]) {
      const column: LayoutTab = {
        id: "tab:column",
        pinned: false,
        root: {
          id: "split:column",
          kind: "split",
          axis: "vertical",
          weights: [0.5, 0.5],
          children,
        },
      };
      const seam = project(column).seams.find(entry => entry.id === "split:column:0");
      if (seam === undefined) throw new Error("expected the column seam");
      // The collapsed side never goes under 36px of switcher + 220px of member.
      const pairIndex = children[0] === pair ? 0 : 1;
      expect(pairIndex === 0 ? seam.minValue : 907 - seam.maxValue).toBeGreaterThanOrEqual(256);

      // Every drag, in half-pixel steps and past both bounds, previews a layout that keeps the
      // column side by side: its seam is still there and only the pair is one at a time.
      for (let deltaPx = -500; deltaPx <= 500; deltaPx += 0.5) {
        const previewed = project(applySeamPreviewToTab(column, seam, deltaPx));
        expect(previewed.collapsed.map(entry => entry.splitId)).toEqual(["split:pair"]);
        expect(previewed.seams.map(entry => entry.id)).toEqual(["split:column:0"]);
      }
    }
  });

  it("Should keep a three-member split side by side at every frame a drag asks for when a fractional shift would give the third member's spare pixel away", () => {
    // Largest-remainder allocation over an 820px row with 220px floors, raw shares
    // 300.3 | 300.3 | 219.4: floors 300 + 300 + 219 = 819, the one spare pixel goes to the largest
    // fraction (the third member's .4), so 300 | 300 | 220 and every member fits.
    // Shifted by +0.5 across the first seam: 300.8 | 299.8 | 219.4, floors 300 + 299 + 219 = 818,
    // the two spare pixels go to the two .8 fractions, so 301 | 300 | 219: the third member drops
    // under its floor while the seam is 80px from either neighbour's minimum. A whole-pixel shift
    // keeps the fractions at .3 | .3 | .4, so the pixel stays with the third member.
    const row: LayoutTab = {
      id: "tab:row",
      pinned: false,
      root: {
        id: "split:row",
        kind: "split",
        axis: "horizontal",
        weights: [300.3 / 820, 300.3 / 820, 219.4 / 820],
        children: [
          { id: "leaf:a", kind: "leaf", windowId: "window:a" },
          { id: "leaf:b", kind: "leaf", windowId: "window:b" },
          { id: "leaf:c", kind: "leaf", windowId: "window:c" },
        ],
      },
    };
    const input = (tab: LayoutTab) => ({
      tab,
      workArea: { x: 0, y: 0, w: 820, h: 700 },
      gaps: { inner: 0, top: 0, right: 0, bottom: 0, left: 0 },
      minimums: {
        "window:a": { width: 220, height: 220 },
        "window:b": { width: 220, height: 220 },
        "window:c": { width: 220, height: 220 },
      },
    });
    const seam = projectTab(input(row)).seams.find(entry => entry.id === "split:row:0");
    if (seam === undefined) throw new Error("expected the first row seam");
    // 300px either side of the seam over 220px floors: 80px of travel each way.
    expect([seam.minValue - seam.value, seam.maxValue - seam.value]).toEqual([-80, 80]);
    // Unguarded, half a pixel is enough to show the row one member at a time.
    expect(projectTab(input(applySeamPreviewToTab(row, seam, 0.5))).collapsed).toHaveLength(1);

    // Every frame, in half-pixel steps and past both bounds, shows the nearest shift toward 0 the
    // row lays out side by side at: the request itself on whole pixels, the whole pixel inside it
    // on half pixels, and the bound past it.
    for (let requestedPx = -120; requestedPx <= 120; requestedPx += 0.5) {
      const admitted = admitSeamShift(input(row), seam, requestedPx);
      if (admitted === null) throw new Error("expected the first row seam");
      const bounded = Math.min(80, Math.max(-80, requestedPx));
      expect(admitted.shiftPx).toBeCloseTo(Math.trunc(bounded), 9);
      const frame = projectTab(input(applySeamPreviewToTab(row, admitted.seam, admitted.shiftPx)));
      expect(frame.collapsed).toEqual([]);
      expect(frame.seams.map(entry => entry.id)).toEqual(["split:row:0", "split:row:1"]);
    }
  });

  it("Should preserve unaffected branch identities when one split changes", () => {
    const base = tab();

    const adjusted = applySeamPreviewToTab(base, SEAM, 99.2);

    expect(adjusted).not.toBe(base);
    expect(adjusted.root).not.toBe(base.root);
    expect(adjusted.root.kind === "split" ? adjusted.root.children[1] : null).toBe(
      UNTOUCHED_BRANCH
    );
  });
});
