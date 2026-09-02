// Suite: structural projected seams
// Invariant: WM-WEB-003 — every split seam is split_id + boundary_index and owns every descendant
// of the two adjacent branches, with resize bounds derived from real branch minimums; abutting
// island frames additionally expose exactly one frame seam per shared boundary line whose edits
// atomically move every island edge on that line.
// Boundary IN: seams derived by pure layout projection.
// Boundary OUT: pointer/keyboard dispatch to the daemon layout.resize / layout.frame_resize commands.
import { describe, expect, it } from "vitest";

import { applyFrameSeamPreviewToDesktop, frameSeamEdits } from "../frame-seams";
import { projectLayout } from "../layout-projection";
import type {
  LayoutDesktop,
  LayoutGroup,
  LayoutProjectionInput,
  NormalizedRect,
} from "../window-manager-types";

function nestedDesktop(): LayoutDesktop {
  return {
    id: "desktop:main",
    name: "Main",
    order: 0,
    floating: [],
    floatingStacks: [],
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

function islandGroup(id: string, windowId: string, frame: NormalizedRect): LayoutGroup {
  return { id, frame, root: { id: `leaf:${id}`, kind: "leaf", windowId } };
}

function islandDesktop(groups: LayoutGroup[]): LayoutDesktop {
  return {
    id: "desktop:islands",
    name: "Islands",
    order: 0,
    floating: [],
    floatingStacks: [],
    groups,
  };
}

function islandProjectionInput(groups: LayoutGroup[]): LayoutProjectionInput {
  return {
    revision: 4,
    desktop: islandDesktop(groups),
    workArea: { x: 0, y: 0, w: 1000, h: 700 },
    gaps: { inner: 8, top: 0, right: 0, bottom: 0, left: 0 },
    minimums: {
      "window:a": { width: 100, height: 80 },
      "window:b": { width: 100, height: 80 },
      "window:c": { width: 100, height: 80 },
    },
  };
}

describe("projected frame seams", () => {
  it("Should expose one draggable boundary between abutting islands with zero-sum edits", () => {
    const groups = [
      islandGroup("group:left", "window:a", { x: 0, y: 0, w: 0.5, h: 1 }),
      islandGroup("group:right", "window:b", { x: 0.5, y: 0, w: 0.5, h: 1 }),
    ];
    const projection = projectLayout(islandProjectionInput(groups));

    expect(projection.seams).toHaveLength(0);
    expect(projection.frameSeams).toHaveLength(1);
    const seam = projection.frameSeams[0];
    expect(seam).toMatchObject({
      orientation: "vertical",
      line: 0.5,
      leadingGroupIds: ["group:left"],
      trailingGroupIds: ["group:right"],
      leadingWindowIds: ["window:a"],
      trailingWindowIds: ["window:b"],
      axisSpan: 1000,
    });
    if (seam === undefined) throw new Error("missing frame seam");

    const edits = frameSeamEdits(groups, seam, 100);
    expect(edits).toEqual([
      { groupId: "group:left", frame: { x: 0, y: 0, w: 0.6, h: 1 } },
      { groupId: "group:right", frame: { x: 0.6, y: 0, w: 0.4, h: 1 } },
    ]);
  });

  it("Should move every island sharing the boundary line through one seam", () => {
    const groups = [
      islandGroup("group:a", "window:a", { x: 0, y: 0, w: 0.5, h: 1 }),
      islandGroup("group:b", "window:b", { x: 0.5, y: 0, w: 0.5, h: 0.5 }),
      islandGroup("group:c", "window:c", { x: 0.5, y: 0.5, w: 0.5, h: 0.5 }),
    ];
    const projection = projectLayout(islandProjectionInput(groups));

    const vertical = projection.frameSeams.filter(seam => seam.orientation === "vertical");
    expect(vertical).toHaveLength(1);
    const seam = vertical[0];
    expect(seam?.leadingGroupIds).toEqual(["group:a"]);
    expect(seam?.trailingGroupIds).toEqual(["group:b", "group:c"]);
    if (seam === undefined) throw new Error("missing frame seam");

    const edits = frameSeamEdits(groups, seam, -100);
    expect(edits).toEqual([
      { groupId: "group:a", frame: { x: 0, y: 0, w: 0.4, h: 1 } },
      { groupId: "group:b", frame: { x: 0.4, y: 0, w: 0.6, h: 0.5 } },
      { groupId: "group:c", frame: { x: 0.4, y: 0.5, w: 0.6, h: 0.5 } },
    ]);
    // The b|c horizontal boundary stays independently draggable.
    expect(
      projection.frameSeams.filter(candidate => candidate.orientation === "horizontal")
    ).toHaveLength(1);
  });

  it("Should keep the seam identity stable while a live preview moves the line", () => {
    const input = islandProjectionInput([
      islandGroup("group:left", "window:a", { x: 0, y: 0, w: 0.5, h: 1 }),
      islandGroup("group:right", "window:b", { x: 0.5, y: 0, w: 0.5, h: 1 }),
    ]);
    const base = projectLayout(input);
    const seam = base.frameSeams[0];
    if (seam === undefined) throw new Error("missing frame seam");

    const previewed = projectLayout({
      ...input,
      desktop: applyFrameSeamPreviewToDesktop(input.desktop, seam, 120),
    });
    const moved = previewed.frameSeams.find(candidate => candidate.id === seam.id);
    // Same identity, moved geometry — the dragged seam element never remounts.
    expect(moved).toBeDefined();
    expect(moved?.line).toBeGreaterThan(seam.line);
  });

  it("Should not link islands that only corner-touch or sit apart", () => {
    const cornerTouch = projectLayout(
      islandProjectionInput([
        islandGroup("group:a", "window:a", { x: 0, y: 0, w: 0.5, h: 0.5 }),
        islandGroup("group:b", "window:b", { x: 0.5, y: 0.5, w: 0.5, h: 0.5 }),
      ])
    );
    expect(cornerTouch.frameSeams).toHaveLength(0);

    const apart = projectLayout(
      islandProjectionInput([
        islandGroup("group:a", "window:a", { x: 0, y: 0, w: 0.4, h: 1 }),
        islandGroup("group:b", "window:b", { x: 0.5, y: 0, w: 0.5, h: 1 }),
      ])
    );
    expect(apart.frameSeams).toHaveLength(0);
  });
});
