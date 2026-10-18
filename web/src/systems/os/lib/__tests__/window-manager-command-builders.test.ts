// Suite: window-manager command builders
// Invariant: each Window › Arrange preset becomes one `layout.arrange` command
// with its daemon arrangement and anchor-first participants, one per frame: a
// deck is named once, by its active tab (the daemon then arranges the whole
// deck), and the anchor's own deck is never named again.
// Owning layer: web/src/systems/os/lib/window-manager-command-builders.ts
import { describe, expect, it } from "vitest";

import type { OsArrangePreset, OsWindow } from "../os-types";
import { arrangeLayoutCommand } from "../window-manager-command-builders";

function osWindow(id: string, overrides: Partial<OsWindow> = {}): OsWindow {
  return {
    id,
    app: "tasks",
    instanceKey: null,
    route: { pathname: "/tasks", search: {} },
    navStack: [],
    pinned: false,
    desktopId: "desktop:a",
    placement: "tiled",
    rect: { x: 0, y: 0, w: 600, h: 400 },
    layer: 1,
    minimized: false,
    zoomed: false,
    groupId: null,
    nodeId: null,
    stackId: null,
    stackActive: true,
    parentAxis: null,
    ...overrides,
  };
}

const ANCHOR = osWindow("w-anchor");
// A tiled two-tab deck (w-2 hidden behind w-1) listed hidden-tab first, then
// three lone windows.
const PEERS = [
  osWindow("w-2", { stackId: "stack:deck", stackActive: false }),
  osWindow("w-1", { stackId: "stack:deck" }),
  osWindow("w-3"),
  osWindow("w-4"),
  osWindow("w-5"),
];

describe("arrangeLayoutCommand", () => {
  it.each<{ preset: OsArrangePreset; arrangement: string; windowIds: string[] }>([
    { preset: "two-up", arrangement: "horizontal", windowIds: ["w-anchor", "w-1"] },
    { preset: "grid", arrangement: "grid", windowIds: ["w-anchor", "w-1", "w-3", "w-4"] },
    {
      preset: "main-stack",
      arrangement: "main_stack",
      windowIds: ["w-anchor", "w-1", "w-3", "w-4", "w-5"],
    },
    {
      preset: "columns",
      arrangement: "horizontal",
      windowIds: ["w-anchor", "w-1", "w-3", "w-4", "w-5"],
    },
  ])(
    "Should arrange $preset as $arrangement with anchor-first participants",
    ({ preset, arrangement, windowIds }) => {
      const command = arrangeLayoutCommand(ANCHOR, PEERS, preset, "group:new");

      expect(command).toEqual({
        commandId: "layout.arrange",
        payload: {
          desktop_id: "desktop:a",
          window_ids: windowIds,
          arrangement,
          frame: { x: 0, y: 0, width: 1, height: 1 },
          group_id: "group:new",
          keep_frames: true,
        },
      });
    }
  );

  it("Should not name the anchor's own deck again, and skip when it is the only frame", () => {
    const anchor = osWindow("w-anchor", { stackId: "stack:own" });
    const sibling = osWindow("w-sibling", { stackId: "stack:own", stackActive: false });

    expect(arrangeLayoutCommand(anchor, [sibling], "columns", "g")).toBeNull();
    expect(
      arrangeLayoutCommand(anchor, [sibling, osWindow("w-peer")], "main-stack", "g")?.payload
    ).toMatchObject({ window_ids: ["w-anchor", "w-peer"] });
  });
});
