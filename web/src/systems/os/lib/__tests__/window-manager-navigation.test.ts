// Suite: window-manager navigation and arrange eligibility
// Invariant: an arrange gathers the anchor, the rest of its tab, then one lead window per other
// tab on the anchor's desktop; directional focus only moves between on-screen windows.
// Owning layer: tab-model selection. Boundary OUT: semantic arrange command construction.
import { describe, expect, it } from "vitest";

import type { LayoutDesktop, LayoutNode, LayoutTab } from "../window-manager-types";
import { arrangeParticipants, directionalFocusTarget } from "../window-manager-navigation";

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
