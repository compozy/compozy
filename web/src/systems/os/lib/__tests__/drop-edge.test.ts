// Suite: drop-edge geometry
// Invariant: every pointer position over a window resolves to exactly one nearest edge, the outer
// 24px band of the content resolves to the tab's root edge, and a drop is withheld where its
// result would put a member below its pixel minimum (UT-031–UT-033).
// Boundary IN: pure point-to-drop math over a tab projection and the simulated drop tree.
// Boundary OUT: pointer transport, the drag store and the daemon's `window.move` reducer.
import { describe, expect, it } from "vitest";

import {
  contentDropAt,
  dropFits,
  insertAtRootEdge,
  insertBesideWindow,
  nearestEdge,
  ROOT_EDGE_BAND_PX,
  rootEdge,
} from "../drop-edge";
import { projectTab } from "../tab-projection";
import type { LayoutNode, LayoutTab, PixelRect } from "../window-manager-types";

const BOX: PixelRect = { x: 100, y: 50, w: 400, h: 200 };
const GAPS = { inner: 0, top: 0, right: 0, bottom: 0, left: 0 };
const WORK_AREA: PixelRect = { x: 0, y: 0, w: 1200, h: 800 };

function leaf(windowId: string): LayoutNode {
  return { id: `leaf:${windowId}`, kind: "leaf", windowId };
}

const TWO_UP: LayoutTab = {
  id: "tab:two",
  pinned: false,
  root: {
    id: "split:two",
    kind: "split",
    axis: "horizontal",
    weights: [0.5, 0.5],
    children: [leaf("a"), leaf("b")],
  },
};

describe("nearestEdge", () => {
  it("UT-031: returns the edge nearest the pointer in each quadrant", () => {
    expect(nearestEdge(BOX, { x: 110, y: 150 })).toBe("left");
    expect(nearestEdge(BOX, { x: 490, y: 150 })).toBe("right");
    expect(nearestEdge(BOX, { x: 300, y: 55 })).toBe("top");
    expect(nearestEdge(BOX, { x: 300, y: 245 })).toBe("bottom");
    // Measured as a share of each side, so a wide window still offers top and bottom.
    expect(nearestEdge(BOX, { x: 200, y: 60 })).toBe("top");
  });

  it("UT-032: the exact center and the diagonals resolve to one deterministic edge", () => {
    expect(nearestEdge(BOX, { x: 300, y: 150 })).toBe("left");
    expect(nearestEdge(BOX, { x: 140, y: 70 })).toBe("left");
    expect(nearestEdge(BOX, { x: 460, y: 70 })).toBe("right");
    expect(nearestEdge(BOX, { x: 460, y: 230 })).toBe("right");
    expect(nearestEdge(BOX, { x: 300, y: 150 })).toBe(nearestEdge(BOX, { x: 300, y: 150 }));
  });
});

describe("content drops", () => {
  const projection = projectTab({ tab: TWO_UP, workArea: WORK_AREA, gaps: GAPS });

  it("UT-032: a point within the outer band returns the tab's root edge", () => {
    expect(rootEdge(WORK_AREA, { x: ROOT_EDGE_BAND_PX, y: 400 })).toBe("left");
    expect(rootEdge(WORK_AREA, { x: ROOT_EDGE_BAND_PX + 1, y: 400 })).toBeNull();
    expect(contentDropAt(projection, { x: 1190, y: 400 })).toEqual({ kind: "root", edge: "right" });
    expect(contentDropAt(projection, { x: 600, y: 790 })).toEqual({ kind: "root", edge: "bottom" });
  });

  it("UT-031: inside the band a point targets the nearest edge of the window under it", () => {
    expect(contentDropAt(projection, { x: 560, y: 400 })).toEqual({
      kind: "window",
      windowId: "a",
      nodeId: "leaf:a",
      edge: "right",
    });
    expect(contentDropAt(projection, { x: 900, y: 100 })).toEqual({
      kind: "window",
      windowId: "b",
      nodeId: "leaf:b",
      edge: "top",
    });
    expect(contentDropAt(projection, { x: 1300, y: 400 })).toBeNull();
  });
});

describe("drop result", () => {
  it("extends a split on the same axis and nests a split on the other one", () => {
    const extended = insertBesideWindow(TWO_UP.root, "b", leaf("c"), "right");
    expect(extended).toMatchObject({ id: "split:two", axis: "horizontal" });
    expect(extended.kind === "split" && extended.children.map(child => child.id)).toEqual([
      "leaf:a",
      "leaf:b",
      "leaf:c",
    ]);
    const nested = insertBesideWindow(TWO_UP.root, "b", leaf("c"), "top");
    expect(nested.kind === "split" && nested.children[1]).toMatchObject({
      kind: "split",
      axis: "vertical",
      weights: [0.5, 0.5],
    });
    const rooted = insertAtRootEdge(TWO_UP.root, leaf("c"), "bottom");
    expect(rooted).toMatchObject({ kind: "split", axis: "vertical", weights: [0.5, 0.5] });
  });

  it("UT-033: withholds a drop whose result puts a member below its pixel minimum", () => {
    const minimums = {
      a: { width: 300, height: 200 },
      b: { width: 300, height: 200 },
      c: { width: 300, height: 200 },
    };
    const narrow: PixelRect = { x: 0, y: 0, w: 800, h: 800 };
    const base = { tab: TWO_UP, incoming: leaf("c"), gaps: GAPS, minimums, workArea: narrow };
    // A third column of 800px leaves 266px each: below the 300px minimum.
    expect(dropFits({ ...base, drop: { kind: "root", edge: "right" } })).toBe(false);
    expect(
      dropFits({
        ...base,
        drop: { kind: "window", windowId: "b", nodeId: "leaf:b", edge: "right" },
      })
    ).toBe(false);
    // Stacking under b leaves 400×400 for each of them: offered.
    expect(
      dropFits({
        ...base,
        drop: { kind: "window", windowId: "b", nodeId: "leaf:b", edge: "bottom" },
      })
    ).toBe(true);
    // A head moved within its own split frees its place first.
    expect(
      dropFits({
        ...base,
        incoming: leaf("a"),
        leavingWindowId: "a",
        drop: { kind: "window", windowId: "b", nodeId: "leaf:b", edge: "bottom" },
      })
    ).toBe(true);
  });

  it("UT-033: a tab that already shows one member at a time takes no member that cannot fit", () => {
    const minimums = {
      a: { width: 400, height: 200 },
      b: { width: 400, height: 200 },
      c: { width: 400, height: 200 },
    };
    // 600px cannot hold a and b side by side: the split already shows one member at a time.
    const collapsedArea: PixelRect = { x: 0, y: 0, w: 600, h: 800 };
    const base = { tab: TWO_UP, incoming: leaf("c"), gaps: GAPS, minimums };
    expect(
      projectTab({ tab: TWO_UP, workArea: collapsedArea, gaps: GAPS, minimums }).collapsed
    ).not.toEqual([]);
    expect(
      dropFits({ ...base, workArea: collapsedArea, drop: { kind: "root", edge: "right" } })
    ).toBe(false);
    expect(
      dropFits({
        ...base,
        workArea: collapsedArea,
        drop: { kind: "window", windowId: "b", nodeId: "leaf:b", edge: "right" },
      })
    ).toBe(false);
  });

  it("UT-033: a shortfall the tab already had is kept by identity when the new member fits", () => {
    const tab: LayoutTab = {
      id: "tab:mixed",
      pinned: false,
      root: {
        id: "split:root",
        kind: "split",
        axis: "horizontal",
        weights: [0.5, 0.5],
        children: [
          leaf("a"),
          {
            id: "split:v",
            kind: "split",
            axis: "vertical",
            weights: [0.5, 0.5],
            children: [leaf("b"), leaf("c")],
          },
        ],
      },
    };
    const minimums = {
      a: { width: 300, height: 200 },
      b: { width: 300, height: 200 },
      c: { width: 300, height: 200 },
      d: { width: 300, height: 200 },
    };
    // 300px tall: b and c cannot stack, so split:v already shows one member at a time.
    const wide: PixelRect = { x: 0, y: 0, w: 1200, h: 300 };
    const base = { tab, incoming: leaf("d"), gaps: GAPS, minimums, workArea: wide };
    expect(dropFits({ ...base, drop: { kind: "root", edge: "left" } })).toBe(true);
    // Stacking d under a leaves 150px each: d itself would fall short.
    expect(
      dropFits({
        ...base,
        drop: { kind: "window", windowId: "a", nodeId: "leaf:a", edge: "bottom" },
      })
    ).toBe(false);
  });

  it("a reorder inside a split never fails the fit check", () => {
    const tab: LayoutTab = {
      id: "tab:three",
      pinned: false,
      root: {
        id: "split:three",
        kind: "split",
        axis: "horizontal",
        weights: [0.2, 0.5, 0.3],
        children: [leaf("a"), leaf("b"), leaf("c")],
      },
    };
    const minimums = {
      a: { width: 300, height: 200 },
      b: { width: 300, height: 200 },
      c: { width: 300, height: 200 },
      d: { width: 300, height: 200 },
    };
    // 800px cannot hold three 300px members: the split already shows one member at a time.
    const narrow: PixelRect = { x: 0, y: 0, w: 800, h: 800 };
    expect(projectTab({ tab, workArea: narrow, gaps: GAPS, minimums }).collapsed).toEqual([
      expect.objectContaining({ splitId: "split:three" }),
    ]);
    const head = { tab, gaps: GAPS, minimums, workArea: narrow, incoming: leaf("a") };
    const reorder = { ...head, leavingWindowId: "a" };

    // a moves beside a sibling along the split's own axis, or to the root edge of that axis.
    expect(
      dropFits({
        ...reorder,
        drop: { kind: "window", windowId: "c", nodeId: "leaf:c", edge: "right" },
      })
    ).toBe(true);
    expect(dropFits({ ...reorder, drop: { kind: "root", edge: "right" } })).toBe(true);
    // A new member still cannot join a split that cannot hold it.
    expect(
      dropFits({
        ...head,
        incoming: leaf("d"),
        drop: { kind: "window", windowId: "c", nodeId: "leaf:c", edge: "right" },
      })
    ).toBe(false);
  });
});
