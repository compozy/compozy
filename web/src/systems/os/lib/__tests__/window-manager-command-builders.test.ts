// Suite: window-manager command builders
// Invariant: every tab-model gesture becomes one daemon command in its v6 wire shape: opens are
// tabs (after a named window's tab, or a split beside focus), closes never send `minimize`, tab
// reorder/activate use `window.tab.*`, moves carry `target_tab_id`/`move_group`, and an arrange
// preset becomes one `layout.arrange` with no retired frame fields; unsplit, tile-to-root-edge,
// split-with and move left/right are `window.move` / `window.tab.reorder` (US-010, US-012).
// Owning layer: web/src/systems/os/lib/window-manager-command-builders.ts
import { describe, expect, it } from "vitest";

import type { OsArrangePreset } from "../os-types";
import {
  activateTabCommand,
  adjacentTabIndex,
  arrangeLayoutCommand,
  balanceLayoutCommand,
  closeWindowCommand,
  moveWindowCommand,
  openWindowCommand,
  reorderTabCommand,
  splitWithCommand,
  tileWindowCommand,
  unsplitWindowCommand,
} from "../window-manager-command-builders";

describe("openWindowCommand", () => {
  it("Should open a tab with no retired floating fields", () => {
    const command = openWindowCommand(
      { app: "tasks", route: { pathname: "/tasks", search: {} } },
      "window:new",
      "desktop:a"
    );

    expect(command).toEqual({
      commandId: "window.open",
      payload: {
        window: {
          id: "window:new",
          app: "tasks",
          route: { pathname: "/tasks", search: {} },
          desktop_id: "desktop:a",
        },
      },
    });
  });

  it("Should open after a named window's tab, or split beside focus", () => {
    expect(
      openWindowCommand({ app: "tasks", afterWindowId: "window:a" }, "w", "desktop:a").payload
    ).toMatchObject({ window: { stack_target_window_id: "window:a" } });
    expect(
      openWindowCommand({ app: "tasks", placement: "split" }, "w", "desktop:a").payload
    ).toMatchObject({ window: { insert_tiled: true } });
  });
});

describe("tab commands", () => {
  it("Should close by scope without sending minimize", () => {
    expect(closeWindowCommand("w").payload).toEqual({ window_id: "w" });
    expect(closeWindowCommand("w", "left").payload).toEqual({ window_id: "w", scope: "left" });
  });

  it("Should reorder and activate through window.tab commands", () => {
    expect(reorderTabCommand("w", 2)).toMatchObject({
      commandId: "window.tab.reorder",
      payload: { window_id: "w", index: 2 },
    });
    expect(activateTabCommand("w")).toMatchObject({
      commandId: "window.tab.activate",
      payload: { window_id: "w" },
    });
  });

  it("Should move a whole tab to another desktop's bar", () => {
    expect(
      moveWindowCommand("w", {
        destinationDesktopId: "desktop:b",
        placement: "tab",
        moveGroup: true,
      }).payload
    ).toEqual({
      window_id: "w",
      destination_desktop_id: "desktop:b",
      placement: "tab",
      move_group: true,
    });
    expect(
      moveWindowCommand("w", {
        destinationDesktopId: "desktop:a",
        targetTabId: "tab:x",
        placement: "right",
      }).payload
    ).toMatchObject({ target_tab_id: "tab:x", placement: "right", move_group: false });
  });
});

describe("arrangeLayoutCommand", () => {
  it.each<{ preset: OsArrangePreset; arrangement: string }>([
    { preset: "two-up", arrangement: "horizontal" },
    { preset: "grid", arrangement: "grid" },
    { preset: "main-stack", arrangement: "main_stack" },
    { preset: "columns", arrangement: "horizontal" },
  ])("Should arrange $preset as $arrangement", ({ preset, arrangement }) => {
    expect(arrangeLayoutCommand("desktop:a", ["w-1", "w-2"], preset)).toEqual({
      commandId: "layout.arrange",
      payload: { desktop_id: "desktop:a", window_ids: ["w-1", "w-2"], arrangement },
    });
  });

  it("Should skip an arrange with a single participant", () => {
    expect(arrangeLayoutCommand("desktop:a", ["w-1"], "columns")).toBeNull();
  });
});

describe("balanceLayoutCommand", () => {
  it("Should balance a tab or a split and refuse an empty target", () => {
    expect(balanceLayoutCommand({ tabId: "tab:a" })?.payload).toEqual({ tab_id: "tab:a" });
    expect(balanceLayoutCommand({ splitId: "split:a" })?.payload).toEqual({
      split_id: "split:a",
    });
    expect(balanceLayoutCommand({})).toBeNull();
  });
});

describe("split commands", () => {
  const member = { id: "w-b", desktopId: "desktop:a", tabId: "tab:ab", inSplit: true };

  it("Should unsplit a member into its own tab after its source tab [US-010.AC-1]", () => {
    expect(unsplitWindowCommand(member)).toEqual({
      commandId: "window.move",
      payload: {
        window_id: "w-b",
        destination_desktop_id: "desktop:a",
        placement: "tab",
        move_group: false,
      },
      rebase: { windowId: "w-b", tabId: "tab:ab" },
    });
    // The only window of its tab has nothing to unsplit (US-010.EC-1).
    expect(unsplitWindowCommand({ ...member, inSplit: false })).toBeNull();
  });

  it("Should tile a member to its own tab's root edge [US-012.AC-2]", () => {
    expect(tileWindowCommand(member, "left")?.payload).toEqual({
      window_id: "w-b",
      destination_desktop_id: "desktop:a",
      target_tab_id: "tab:ab",
      placement: "left",
      move_group: false,
    });
    expect(tileWindowCommand({ ...member, inSplit: false }, "top")).toBeNull();
  });

  it("Should join the chosen tab whole on the host tab's right [US-012.AC-3]", () => {
    expect(splitWithCommand("desktop:a", "tab:host", "w-chosen").payload).toEqual({
      window_id: "w-chosen",
      destination_desktop_id: "desktop:a",
      target_tab_id: "tab:host",
      placement: "right",
      move_group: true,
    });
  });
});

describe("adjacentTabIndex", () => {
  const tab = (id: string, pinned = false) => ({
    id,
    pinned,
    root: { id: `leaf:${id}`, kind: "leaf" as const, windowId: `w-${id}` },
  });
  const tabs = [tab("pin", true), tab("a"), tab("b"), tab("c")];

  it("Should step one place within the tab's range and stop at its edges", () => {
    expect(adjacentTabIndex(tabs, "b", "left")).toBe(1);
    expect(adjacentTabIndex(tabs, "b", "right")).toBe(3);
    // A regular tab never moves among the pins; the last tab has no Move right.
    expect(adjacentTabIndex(tabs, "a", "left")).toBeNull();
    expect(adjacentTabIndex(tabs, "c", "right")).toBeNull();
    // A pinned tab stays among the pins.
    expect(adjacentTabIndex(tabs, "pin", "right")).toBeNull();
    expect(adjacentTabIndex(tabs, "missing", "left")).toBeNull();
  });
});
