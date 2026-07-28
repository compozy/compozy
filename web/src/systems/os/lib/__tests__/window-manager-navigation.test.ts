// Suite: window-manager navigation and arrange eligibility
// Invariant: an arrange command can only consume visible peers from its anchor desktop.
// Owning layer: projected-window selection. Boundary OUT: semantic arrange command construction.
import { describe, expect, it } from "vitest";

import type { OsWindow } from "../os-types";
import { arrangePeerWindows } from "../window-manager-navigation";

function windowFixture(id: string, desktopId: string, layer: number, minimized = false): OsWindow {
  return {
    id,
    app: "tasks",
    instanceKey: null,
    route: { pathname: "/tasks", search: {} },
    desktopId,
    placement: "floating",
    rect: { x: 40, y: 40, w: 720, h: 480 },
    layer,
    minimized,
    groupId: null,
    nodeId: null,
    stackId: null,
    stackActive: true,
    parentAxis: null,
  };
}

describe("arrangePeerWindows", () => {
  it("Should exclude minimized and cross-desktop windows and preserve layer order", () => {
    const windows = {
      anchor: windowFixture("anchor", "desktop:a", 1),
      later: windowFixture("later", "desktop:a", 2),
      front: windowFixture("front", "desktop:a", 5),
      minimized: windowFixture("minimized", "desktop:a", 9, true),
      remote: windowFixture("remote", "desktop:b", 12),
    };

    expect(arrangePeerWindows(windows, "anchor").map(window => window.id)).toEqual([
      "front",
      "later",
    ]);
    expect(arrangePeerWindows(windows, "missing")).toEqual([]);
  });
});
