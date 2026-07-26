// Suite: browser layout projection
// Invariant: WM-WEB-001/002 — normalized topology projects deterministically with one shared gap,
// no non-stack overlap, and impossible minimums adapt temporarily without mutating topology.
// Boundary IN: pure normalized-to-pixel projection.
// Boundary OUT: daemon normalization/validation and rendered window components.
import { describe, expect, it } from "vitest";

import { projectLayout } from "../layout-projection";
import type { LayoutDesktop, LayoutProjectionInput, PixelRect } from "../window-manager-types";

const DESKTOP: LayoutDesktop = {
  id: "desktop:main",
  name: "Main",
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
    },
  ],
};

const INPUT: LayoutProjectionInput = {
  revision: 7,
  desktop: DESKTOP,
  workArea: { x: 10, y: 20, w: 1001, h: 701 },
  gaps: { inner: 9, top: 7, right: 13, bottom: 17, left: 11 },
  minimums: {
    "window:a": { width: 200, height: 180 },
    "window:b": { width: 200, height: 180 },
    "window:c": { width: 200, height: 180 },
  },
};

function overlapArea(a: PixelRect, b: PixelRect): number {
  const width = Math.max(0, Math.min(a.x + a.w, b.x + b.w) - Math.max(a.x, b.x));
  const height = Math.max(0, Math.min(a.y + a.h, b.y + b.h) - Math.max(a.y, b.y));
  return width * height;
}

describe("projectLayout", () => {
  it("Should project nested splits with deterministic rounding and exactly one shared gap", () => {
    const first = projectLayout(INPUT);
    const second = projectLayout(INPUT);

    expect(second).toEqual(first);
    expect(first.windows.map(window => [window.windowId, window.rect])).toEqual([
      ["window:a", { x: 21, y: 27, w: 484, h: 677 }],
      ["window:b", { x: 514, y: 27, w: 484, h: 334 }],
      ["window:c", { x: 514, y: 370, w: 484, h: 334 }],
    ]);
    expect(first.seams.map(seam => seam.rect)).toEqual([
      { x: 514, y: 361, w: 484, h: 9 },
      { x: 505, y: 27, w: 9, h: 677 },
    ]);
    for (const window of first.windows) {
      expect(Object.values(window.rect).every(Number.isFinite)).toBe(true);
      expect(Number.isInteger(window.rect.x)).toBe(true);
      expect(Number.isInteger(window.rect.w)).toBe(true);
    }
    expect(
      overlapArea(
        first.windows[0]?.rect ?? INPUT.workArea,
        first.windows[1]?.rect ?? INPUT.workArea
      )
    ).toBe(0);
    expect(
      overlapArea(
        first.windows[1]?.rect ?? INPUT.workArea,
        first.windows[2]?.rect ?? INPUT.workArea
      )
    ).toBe(0);
  });

  it("Should keep an explicit stack reachable with exactly one active window", () => {
    const projection = projectLayout({
      ...INPUT,
      desktop: {
        id: "desktop:stack",
        name: "Stack",
        order: 0,
        purpose: "standard",
        focusOwner: null,
        floating: [],
        groups: [
          {
            id: "group:stack",
            frame: { x: 0, y: 0, w: 1, h: 1 },
            root: {
              id: "stack:one",
              kind: "stack",
              windowIds: ["window:a", "window:b"],
              activeId: "window:b",
            },
          },
        ],
      },
    });

    expect(projection.stacks).toEqual([
      {
        nodeId: "stack:one",
        groupId: "group:stack",
        kind: "explicit",
        windowIds: ["window:a", "window:b"],
        activeWindowId: "window:b",
        rect: { x: 21, y: 27, w: 977, h: 677 },
      },
    ]);
    expect(
      projection.windows.filter(window => window.active).map(window => window.windowId)
    ).toEqual(["window:b"]);
  });

  it("Should adapt an impossible split to a temporary stack and restore when space permits", () => {
    const twoWindows: LayoutDesktop = {
      id: "desktop:small",
      name: "Small",
      order: 0,
      purpose: "standard",
      focusOwner: null,
      floating: [],
      groups: [
        {
          id: "group:small",
          frame: { x: 0, y: 0, w: 1, h: 1 },
          root: {
            id: "split:small",
            kind: "split",
            axis: "horizontal",
            weights: [1, 1],
            children: [
              { id: "leaf:a", kind: "leaf", windowId: "window:a" },
              { id: "leaf:b", kind: "leaf", windowId: "window:b" },
            ],
          },
        },
      ],
    };
    const topologyBefore = JSON.stringify(twoWindows);
    const base = {
      revision: 2,
      desktop: twoWindows,
      gaps: { inner: 8, top: 0, right: 0, bottom: 0, left: 0 },
      minimums: {
        "window:a": { width: 280, height: 180 },
        "window:b": { width: 280, height: 180 },
      },
      focusedWindowId: "window:b",
    } satisfies Omit<LayoutProjectionInput, "workArea">;

    const small = projectLayout({ ...base, workArea: { x: 0, y: 0, w: 500, h: 400 } });
    expect(small.stacks.map(stack => [stack.kind, stack.windowIds, stack.activeWindowId])).toEqual([
      ["adaptive", ["window:a", "window:b"], "window:b"],
    ]);
    expect(small.windows.map(window => window.rect)).toEqual([
      { x: 0, y: 0, w: 500, h: 400 },
      { x: 0, y: 0, w: 500, h: 400 },
    ]);
    expect(small.seams).toHaveLength(0);
    expect(JSON.stringify(twoWindows)).toBe(topologyBefore);

    const large = projectLayout({ ...base, workArea: { x: 0, y: 0, w: 800, h: 400 } });
    expect(large.stacks).toHaveLength(0);
    expect(large.seams).toHaveLength(1);
    expect(
      overlapArea(
        large.windows[0]?.rect ?? large.workArea,
        large.windows[1]?.rect ?? large.workArea
      )
    ).toBe(0);
    expect(JSON.stringify(twoWindows)).toBe(topologyBefore);
  });

  it("Should adapt a weighted split when any allocated child falls below its minimum", () => {
    const weighted: LayoutDesktop = {
      id: "desktop:weighted",
      name: "Weighted",
      order: 0,
      purpose: "standard",
      focusOwner: null,
      floating: [],
      groups: [
        {
          id: "group:weighted",
          frame: { x: 0, y: 0, w: 1, h: 1 },
          root: {
            id: "split:weighted",
            kind: "split",
            axis: "horizontal",
            weights: [9, 1],
            children: [
              { id: "leaf:a", kind: "leaf", windowId: "window:a" },
              { id: "leaf:b", kind: "leaf", windowId: "window:b" },
            ],
          },
        },
      ],
    };

    const projection = projectLayout({
      revision: 3,
      desktop: weighted,
      workArea: { x: 0, y: 0, w: 600, h: 400 },
      gaps: { inner: 8, top: 0, right: 0, bottom: 0, left: 0 },
      minimums: {
        "window:a": { width: 280, height: 180 },
        "window:b": { width: 280, height: 180 },
      },
      focusedWindowId: "window:b",
    });

    expect(projection.stacks).toEqual([
      expect.objectContaining({
        kind: "adaptive",
        windowIds: ["window:a", "window:b"],
        activeWindowId: "window:b",
      }),
    ]);
    expect(projection.seams).toHaveLength(0);
    expect(projection.windows.map(window => window.rect)).toEqual([
      { x: 0, y: 0, w: 600, h: 400 },
      { x: 0, y: 0, w: 600, h: 400 },
    ]);
  });
});
