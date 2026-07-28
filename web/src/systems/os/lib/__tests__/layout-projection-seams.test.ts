// Suite: structural projected seams
// Invariant: WM-WEB-003 — every seam is split_id + boundary_index and owns every descendant of
// the two adjacent branches, with resize bounds derived from real branch minimums.
// Boundary IN: seams derived by pure layout projection.
// Boundary OUT: pointer/keyboard dispatch to the daemon layout.resize command.
import { describe, expect, it } from "vitest";

import { projectLayout } from "../layout-projection";
import type { LayoutDesktop, LayoutProjectionInput } from "../window-manager-types";

function nestedDesktop(): LayoutDesktop {
  return {
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
            { id: "leaf:left", kind: "leaf", windowId: "window:left" },
            {
              id: "split:right",
              kind: "split",
              axis: "vertical",
              weights: [1, 1],
              children: [
                { id: "leaf:top", kind: "leaf", windowId: "window:top-right" },
                { id: "leaf:bottom", kind: "leaf", windowId: "window:bottom-right" },
              ],
            },
          ],
        },
      },
    ],
  };
}

function projectionInput(width = 1000): LayoutProjectionInput {
  return {
    revision: 11,
    desktop: nestedDesktop(),
    workArea: { x: 0, y: 0, w: width, h: 700 },
    gaps: { inner: 8, top: 0, right: 0, bottom: 0, left: 0 },
    minimums: {
      "window:left": { width: 280, height: 180 },
      "window:top-right": { width: 280, height: 180 },
      "window:bottom-right": { width: 280, height: 180 },
    },
  };
}

describe("projected structural seams", () => {
  it("Should expose one one-to-many seam for a half beside two quarters", () => {
    const projection = projectLayout(projectionInput());
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
      leadingWindowIds: ["window:left"],
      trailingWindowIds: ["window:top-right", "window:bottom-right"],
    });
    expect(root?.value).toBeGreaterThanOrEqual(root?.minValue ?? Number.POSITIVE_INFINITY);
    expect(root?.value).toBeLessThanOrEqual(root?.maxValue ?? Number.NEGATIVE_INFINITY);
  });

  it("Should scope a nested boundary to the descendants adjacent to that split", () => {
    const projection = projectLayout(projectionInput());
    const nested = projection.seams.find(seam => seam.id === "split:right:0");

    expect(nested?.orientation).toBe("horizontal");
    expect(nested?.rect).toEqual({ x: 504, y: 346, w: 496, h: 8 });
    expect(nested?.leadingWindowIds).toEqual(["window:top-right"]);
    expect(nested?.trailingWindowIds).toEqual(["window:bottom-right"]);
    expect(new Set(projection.seams.map(seam => seam.id)).size).toBe(projection.seams.length);
  });

  it("Should remove interactive seams when the branch adapts to a temporary stack", () => {
    const projection = projectLayout(projectionInput(500));

    expect(projection.stacks.map(stack => stack.kind)).toEqual(["adaptive"]);
    expect(projection.seams).toHaveLength(0);
  });
});
