// Suite: tab split projection
// Invariant: WM-WEB-001/002/003 — a tab's split tree projects deterministically from its weights
// with one shared gap on every boundary and no overlap; every seam is split_id + boundary_index
// with bounds from the adjacent branches' floors; and a split whose members cannot fit (or any
// split in the compact viewport) shows one member at a time without touching the weights (UT-037).
// Boundary IN: pure tab-to-pixel projection.
// Boundary OUT: daemon normalization/validation and rendered split view.
import { describe, expect, it } from "vitest";

import { normalizedWeights, projectTab } from "../tab-projection";
import type { LayoutTab, PixelRect, TabProjectionInput } from "../window-manager-types";

const NESTED_TAB: LayoutTab = {
  id: "tab:main",
  pinned: false,
  root: {
    id: "split:root",
    kind: "split",
    axis: "horizontal",
    weights: [1, 1],
    children: [
      { id: "leaf:a", kind: "leaf", windowId: "window:a" },
      {
        id: "split:right",
        kind: "split",
        axis: "vertical",
        weights: [1, 1],
        children: [
          { id: "leaf:b", kind: "leaf", windowId: "window:b" },
          { id: "leaf:c", kind: "leaf", windowId: "window:c" },
        ],
      },
    ],
  },
};

const MINIMUMS = {
  "window:a": { width: 280, height: 180 },
  "window:b": { width: 280, height: 180 },
  "window:c": { width: 280, height: 180 },
};

function input(workArea: PixelRect, overrides: Partial<TabProjectionInput> = {}) {
  return {
    tab: NESTED_TAB,
    workArea,
    gaps: { inner: 8, top: 0, right: 0, bottom: 0, left: 0 },
    minimums: MINIMUMS,
    ...overrides,
  } satisfies TabProjectionInput;
}

function twoUp(weights: readonly number[]): LayoutTab {
  return {
    id: "tab:two",
    pinned: false,
    root: {
      id: "split:two",
      kind: "split",
      axis: "horizontal",
      weights,
      children: [
        { id: "leaf:a", kind: "leaf", windowId: "window:a" },
        { id: "leaf:b", kind: "leaf", windowId: "window:b" },
      ],
    },
  };
}

function overlapArea(a: PixelRect, b: PixelRect): number {
  const width = Math.max(0, Math.min(a.x + a.w, b.x + b.w) - Math.max(a.x, b.x));
  const height = Math.max(0, Math.min(a.y + a.h, b.y + b.h) - Math.max(a.y, b.y));
  return width * height;
}

describe("projectTab layout", () => {
  it("Should project nested splits with deterministic rounding and exactly one shared gap", () => {
    const projectionInput = input(
      { x: 10, y: 20, w: 1001, h: 701 },
      { gaps: { inner: 9, top: 7, right: 13, bottom: 17, left: 11 } }
    );
    const first = projectTab(projectionInput);

    expect(projectTab(projectionInput)).toEqual(first);
    expect(first.tabId).toBe("tab:main");
    expect(first.windows.map(window => [window.windowId, window.rect, window.shown])).toEqual([
      ["window:a", { x: 21, y: 27, w: 484, h: 677 }, true],
      ["window:b", { x: 514, y: 27, w: 484, h: 334 }, true],
      ["window:c", { x: 514, y: 370, w: 484, h: 334 }, true],
    ]);
    expect(first.seams.map(seam => seam.rect)).toEqual([
      { x: 514, y: 361, w: 484, h: 9 },
      { x: 505, y: 27, w: 9, h: 677 },
    ]);
    expect(first.collapsed).toEqual([]);
    const rects = first.windows.map(window => window.rect);
    for (const [left, right] of [
      [0, 1],
      [1, 2],
      [0, 2],
    ] as const) {
      expect(overlapArea(rects[left] ?? first.workArea, rects[right] ?? first.workArea)).toBe(0);
    }
  });

  it("Should fill the axis exactly with largest-remainder pixel allocation", () => {
    const projection = projectTab({
      tab: {
        id: "tab:thirds",
        pinned: false,
        root: {
          id: "split:thirds",
          kind: "split",
          axis: "horizontal",
          weights: [1, 1, 1],
          children: ["window:a", "window:b", "window:c"].map(windowId => ({
            id: `leaf:${windowId}`,
            kind: "leaf" as const,
            windowId,
          })),
        },
      },
      workArea: { x: 0, y: 0, w: 100, h: 50 },
      gaps: { inner: 0, top: 0, right: 0, bottom: 0, left: 0 },
    });

    expect(projection.windows.map(window => [window.rect.x, window.rect.w])).toEqual([
      [0, 34],
      [34, 33],
      [67, 33],
    ]);
  });

  it("Should fall back to equal shares for malformed weights", () => {
    const tab = twoUp([0, 1]);
    if (tab.root.kind !== "split") throw new Error("expected split root");

    expect(normalizedWeights(tab.root)).toEqual([0.5, 0.5]);
  });

  it("Should project a single-window tab as one shown window with no seams", () => {
    const projection = projectTab({
      tab: {
        id: "tab:one",
        pinned: true,
        root: { id: "leaf:a", kind: "leaf", windowId: "window:a" },
      },
      workArea: { x: 0, y: 0, w: 800, h: 600 },
      gaps: { inner: 8, top: 0, right: 0, bottom: 0, left: 0 },
    });

    expect(projection.windows).toEqual([
      {
        windowId: "window:a",
        nodeId: "leaf:a",
        rect: { x: 0, y: 0, w: 800, h: 600 },
        shown: true,
        parentAxis: null,
      },
    ]);
    expect(projection.seams).toEqual([]);
  });
});

describe("projectTab seams", () => {
  it("Should expose one one-to-many seam for a half beside two quarters", () => {
    const projection = projectTab(input({ x: 0, y: 0, w: 1000, h: 700 }));
    const root = projection.seams.find(seam => seam.id === "split:root:0");

    expect(root).toEqual({
      id: "split:root:0",
      splitId: "split:root",
      boundaryIndex: 0,
      orientation: "vertical",
      rect: { x: 496, y: 0, w: 8, h: 700 },
      value: 500,
      minValue: 284,
      maxValue: 716,
      axisSpan: 992,
      leadingWeight: 0.5,
      trailingWeight: 0.5,
      leadingWindowIds: ["window:a"],
      trailingWindowIds: ["window:b", "window:c"],
    });
  });

  it("Should scope a nested boundary to the descendants adjacent to that split", () => {
    const projection = projectTab(input({ x: 0, y: 0, w: 1000, h: 700 }));
    const nested = projection.seams.find(seam => seam.id === "split:right:0");

    expect(nested?.orientation).toBe("horizontal");
    expect(nested?.rect).toEqual({ x: 504, y: 346, w: 496, h: 8 });
    expect(nested?.leadingWindowIds).toEqual(["window:b"]);
    expect(nested?.trailingWindowIds).toEqual(["window:c"]);
    expect(new Set(projection.seams.map(seam => seam.id)).size).toBe(projection.seams.length);
  });
});

describe("projectTab collapsed splits", () => {
  it("UT-037: Should show one member at a time when members cannot fit, weights untouched", () => {
    const tabBefore = JSON.stringify(NESTED_TAB);
    const projection = projectTab(
      input({ x: 0, y: 0, w: 500, h: 700 }, { focusOrder: ["window:c", "window:a"] })
    );

    expect(projection.collapsed).toEqual([
      {
        splitId: "split:root",
        windowIds: ["window:a", "window:b", "window:c"],
        shownWindowId: "window:c",
        rect: { x: 0, y: 0, w: 500, h: 700 },
      },
    ]);
    expect(projection.windows.map(window => [window.windowId, window.rect, window.shown])).toEqual([
      ["window:a", { x: 0, y: 0, w: 500, h: 700 }, false],
      ["window:b", { x: 0, y: 0, w: 500, h: 700 }, false],
      ["window:c", { x: 0, y: 0, w: 500, h: 700 }, true],
    ]);
    expect(projection.seams).toEqual([]);
    expect(projection.diagnostics).toEqual([
      expect.objectContaining({ code: "adaptive-stack", nodeId: "split:root" }),
    ]);
    expect(JSON.stringify(NESTED_TAB)).toBe(tabBefore);
  });

  it("UT-037: Should collapse only the split that cannot fit, showing its first member", () => {
    // 300px tall: the left window fits beside the right branch, but the right branch's two
    // 180px-tall members cannot stack, so only that branch shows one member at a time.
    const projection = projectTab(input({ x: 0, y: 0, w: 1000, h: 300 }));

    expect(projection.collapsed).toEqual([
      {
        splitId: "split:right",
        windowIds: ["window:b", "window:c"],
        shownWindowId: "window:b",
        rect: { x: 504, y: 0, w: 496, h: 300 },
      },
    ]);
    expect(projection.windows.map(window => [window.windowId, window.shown])).toEqual([
      ["window:a", true],
      ["window:b", true],
      ["window:c", false],
    ]);
    expect(projection.seams.map(seam => [seam.id, seam.maxValue])).toEqual([["split:root:0", 716]]);
  });

  it("UT-037: Should collapse a weighted split when one member falls below its minimum", () => {
    const tab = twoUp([9, 1]);
    const projection = projectTab({
      tab,
      workArea: { x: 0, y: 0, w: 600, h: 400 },
      gaps: { inner: 8, top: 0, right: 0, bottom: 0, left: 0 },
      minimums: MINIMUMS,
      focusOrder: ["window:b"],
    });

    expect(projection.collapsed).toEqual([expect.objectContaining({ shownWindowId: "window:b" })]);
    expect(projection.seams).toEqual([]);
    expect(tab.root.kind === "split" ? tab.root.weights : null).toEqual([9, 1]);
  });

  it("UT-037: Should show every split one member at a time in the compact viewport", () => {
    const projection = projectTab({
      tab: twoUp([1, 1]),
      workArea: { x: 0, y: 0, w: 1400, h: 800 },
      gaps: { inner: 8, top: 0, right: 0, bottom: 0, left: 0 },
      minimums: MINIMUMS,
      compact: true,
    });

    expect(projection.collapsed).toEqual([
      expect.objectContaining({ splitId: "split:two", shownWindowId: "window:a" }),
    ]);
    const shown = projection.windows.filter(window => window.shown);
    expect(shown.map(window => window.windowId)).toEqual(["window:a"]);
    expect(projection.seams).toEqual([]);
  });

  it("UT-037: Should restore the side-by-side split when space permits again", () => {
    const projection = projectTab({
      tab: twoUp([1, 1]),
      workArea: { x: 0, y: 0, w: 800, h: 400 },
      gaps: { inner: 8, top: 0, right: 0, bottom: 0, left: 0 },
      minimums: MINIMUMS,
    });

    expect(projection.collapsed).toEqual([]);
    expect(projection.seams).toHaveLength(1);
    expect(projection.windows.every(window => window.shown)).toBe(true);
    expect(
      overlapArea(
        projection.windows[0]?.rect ?? projection.workArea,
        projection.windows[1]?.rect ?? projection.workArea
      )
    ).toBe(0);
  });
});
