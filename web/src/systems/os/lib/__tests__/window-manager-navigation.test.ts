// Suite: window-manager navigation and arrange eligibility
// Invariant: an arrange gathers the anchor, the rest of its tab, then one lead window per other
// tab on the anchor's desktop; directional focus only moves between on-screen windows; a
// directional swap picks the nearest on-screen neighbour of the active tab and never wraps.
// Owning layer: tab-model selection. Boundary OUT: semantic arrange command construction.
import { describe, expect, it } from "vitest";

import type { LayoutDesktop, LayoutNode, LayoutTab } from "../window-manager-types";
import {
  activeTabCandidates,
  arrangeParticipants,
  directionalFocusTarget,
  directionalSwapTarget,
} from "../window-manager-navigation";

function leaf(windowId: string): LayoutNode {
  return { id: `node:${windowId}`, kind: "leaf", windowId };
}

function tab(id: string, root: LayoutNode, pinned = false): LayoutTab {
  return { id, pinned, root };
}

const desktop: LayoutDesktop = {
  id: "desktop:a",
  name: "Main",
  order: 0,
  activeTabId: "tab:split",
  tabs: [
    tab("tab:pinned", leaf("pinned"), true),
    tab("tab:split", {
      id: "split:1",
      kind: "split",
      axis: "horizontal",
      weights: [0.5, 0.5],
      children: [leaf("anchor"), leaf("beside")],
    }),
    tab("tab:pair", {
      id: "split:2",
      kind: "split",
      axis: "vertical",
      weights: [0.5, 0.5],
      children: [leaf("pair-a"), leaf("pair-b")],
    }),
    tab("tab:solo", leaf("solo")),
  ],
};

describe("arrangeParticipants", () => {
  it("Should gather the anchor tab and one lead window per other tab in bar order", () => {
    expect(arrangeParticipants(desktop, "anchor", ["pair-b"], "columns")).toEqual([
      "anchor",
      "beside",
      "pinned",
      "pair-b",
      "solo",
    ]);
  });

  it("Should cap two-up at two windows and grid at four", () => {
    expect(arrangeParticipants(desktop, "anchor", [], "two-up")).toEqual(["anchor", "beside"]);
    expect(arrangeParticipants(desktop, "anchor", [], "grid")).toEqual([
      "anchor",
      "beside",
      "pinned",
      "pair-a",
    ]);
  });

  it("Should return nothing for a window the desktop does not hold", () => {
    expect(arrangeParticipants(desktop, "missing", [], "columns")).toEqual([]);
  });
});

describe("directionalFocusTarget", () => {
  const candidates = [
    { id: "left", rect: { x: 0, y: 0, w: 100, h: 100 } },
    { id: "right", rect: { x: 100, y: 0, w: 100, h: 100 } },
  ];

  it("Should move to the nearest window in the direction and wrap only when asked", () => {
    expect(
      directionalFocusTarget({ candidates, focusedId: "left", direction: "right", wrap: false })
    ).toBe("right");
    expect(
      directionalFocusTarget({ candidates, focusedId: "right", direction: "right", wrap: false })
    ).toBeNull();
    expect(
      directionalFocusTarget({ candidates, focusedId: "right", direction: "right", wrap: true })
    ).toBe("left");
  });

  it("Should move within a stacked column instead of jumping to a full-height neighbor", () => {
    // A full-height window beside a stacked pair: Split right, then Split down on the right.
    const stacked = [
      { id: "tall", rect: { x: 0, y: 0, w: 100, h: 200 } },
      { id: "top", rect: { x: 100, y: 0, w: 100, h: 100 } },
      { id: "bottom", rect: { x: 100, y: 100, w: 100, h: 100 } },
    ];
    const move = (focusedId: string, direction: "left" | "right" | "up" | "down") =>
      directionalFocusTarget({ candidates: stacked, focusedId, direction, wrap: false });
    expect(move("top", "down")).toBe("bottom");
    expect(move("bottom", "up")).toBe("top");
    expect(move("bottom", "left")).toBe("tall");
    expect(move("top", "up")).toBeNull();
    expect(move("bottom", "down")).toBeNull();
    expect(directionalSwapTarget(stacked, "top", "down")).toBe("bottom");
  });

  it.each([
    {
      name: "a window sharing the source span over a diagonal one, despite its larger gap",
      candidates: [
        { id: "aligned", rect: { x: 200, y: 0, w: 100, h: 100 } },
        { id: "diagonal", rect: { x: 100, y: 100, w: 100, h: 100 } },
      ],
      want: "aligned",
    },
    {
      name: "the nearest edge among aligned windows",
      candidates: [
        { id: "far", rect: { x: 300, y: 0, w: 100, h: 100 } },
        { id: "near", rect: { x: 100, y: 0, w: 100, h: 100 } },
      ],
      want: "near",
    },
    {
      name: "the closest center when edges tie",
      candidates: [
        { id: "offset", rect: { x: 100, y: 60, w: 100, h: 100 } },
        { id: "level", rect: { x: 100, y: 10, w: 100, h: 100 } },
      ],
      want: "level",
    },
    {
      name: "the lower id when everything else ties",
      candidates: [
        { id: "b-right", rect: { x: 100, y: -50, w: 100, h: 100 } },
        { id: "a-right", rect: { x: 100, y: 50, w: 100, h: 100 } },
      ],
      want: "a-right",
    },
  ])("Should prefer $name", ({ candidates, want }) => {
    const source = { id: "source", rect: { x: 0, y: 0, w: 100, h: 100 } };
    const all = [source, ...candidates];
    expect(
      directionalFocusTarget({
        candidates: all,
        focusedId: "source",
        direction: "right",
        wrap: false,
      })
    ).toBe(want);
    expect(directionalSwapTarget(all, "source", "right")).toBe(want);
  });
});

describe("directionalSwapTarget", () => {
  function view(windows: { id: string; x: number; shown?: boolean }[]) {
    return {
      activeDesktopId: "desktop:a",
      activeTabIds: { "desktop:a": "tab:split" },
      projections: {
        "tab:split": {
          windows: windows.map(window => ({
            windowId: window.id,
            nodeId: `node:${window.id}`,
            rect: { x: window.x, y: 0, w: 100, h: 100 },
            shown: window.shown ?? true,
            parentAxis: "horizontal" as const,
          })),
        },
      },
    };
  }
  const three = activeTabCandidates(
    view([
      { id: "left", x: 0 },
      { id: "middle", x: 100 },
      { id: "right", x: 200 },
    ])
  );

  it("Should pick the nearest neighbour on that side and never wrap", () => {
    expect(directionalSwapTarget(three, "left", "right")).toBe("middle");
    expect(directionalSwapTarget(three, "middle", "left")).toBe("left");
    expect(directionalSwapTarget(three, "left", "left")).toBeNull();
    expect(directionalSwapTarget(three, "right", "right")).toBeNull();
    expect(directionalSwapTarget(three, "left", "up")).toBeNull();
  });

  it("Should only consider windows on screen, and nothing from a window that is not", () => {
    const collapsed = activeTabCandidates(
      view([
        { id: "left", x: 0 },
        { id: "hidden", x: 100, shown: false },
      ])
    );
    expect(directionalSwapTarget(collapsed, "left", "right")).toBeNull();
    expect(directionalSwapTarget(collapsed, "hidden", "left")).toBeNull();
    expect(directionalSwapTarget(three, null, "right")).toBeNull();
    const detached = { ...view([{ id: "left", x: 0 }]), activeDesktopId: null };
    expect(activeTabCandidates(detached)).toEqual([]);
  });
});
