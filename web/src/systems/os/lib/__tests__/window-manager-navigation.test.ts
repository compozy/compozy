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
