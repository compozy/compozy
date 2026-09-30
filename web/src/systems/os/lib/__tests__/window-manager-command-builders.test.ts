// Suite: window-manager command builders
// Invariant: each Window › Arrange preset becomes one `layout.arrange` command
// with its daemon arrangement and anchor-first participants; the all-peer
// presets never pull a hidden tab out of its deck.
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
const PEERS = [
  osWindow("w-1"),
  osWindow("w-2", { stackActive: false }),
  osWindow("w-3"),
  osWindow("w-4"),
  osWindow("w-5"),
];

describe("arrangeLayoutCommand", () => {
  it.each<{ preset: OsArrangePreset; arrangement: string; windowIds: string[] }>([
    { preset: "two-up", arrangement: "horizontal", windowIds: ["w-anchor", "w-1"] },
    { preset: "grid", arrangement: "grid", windowIds: ["w-anchor", "w-1", "w-2", "w-3"] },
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
        },
      });
    }
  );

  it("Should skip an all-peer preset when no other window is visible", () => {
    expect(
      arrangeLayoutCommand(ANCHOR, [osWindow("w-hidden", { stackActive: false })], "columns", "g")
    ).toBeNull();
  });
});
