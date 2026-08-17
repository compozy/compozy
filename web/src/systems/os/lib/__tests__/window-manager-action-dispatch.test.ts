// Suite: window-manager action dispatch
// Invariant: registry action ids map to exactly one controller/coordinator call — tab targets
// resolve against the focused frame's deck order and degrade to quiet no-ops on single-tab
// frames (US-020 AC-3).
// Boundary OUT: keyboard event matching (window-manager-shortcuts suite) and command transport.
import { describe, expect, it, vi } from "vitest";

import { dispatchWindowManagerAction } from "../window-manager-action-dispatch";
import type { OsWindowFrameModel } from "../group-projection";
import type { OsDesktopRuntimeStore, WindowManagerController } from "../os-types";

function frameFixture(overrides: Partial<OsWindowFrameModel> = {}): OsWindowFrameModel {
  return {
    id: "fs-1",
    desktopId: "desktop:a",
    kind: "floating",
    rect: { x: 0, y: 0, w: 600, h: 400 },
    members: ["w-1", "w-2", "w-3"],
    activeWindowId: "w-2",
    stackId: "fs-1",
    minimized: false,
    adapted: false,
    layer: 1,
    zone: null,
    resizableEdges: { left: true, right: true, top: true, bottom: true },
    ...overrides,
  };
}

function contextFixture(frame: OsWindowFrameModel | null, focusedId: string | null) {
  const manager = {
    reopenWindow: vi.fn(() => ({ accepted: true, completion: Promise.resolve(true) })),
    switchDesktop: vi.fn(),
    switchDesktopDirection: vi.fn(),
    createDesktop: vi.fn(),
  } as unknown as WindowManagerController;
  const state = {
    focusedId,
    frames: frame ? { [frame.desktopId]: [frame] } : {},
    windows: {
      "w-1": { id: "w-1", minimized: false },
      "w-2": { id: "w-2", minimized: false },
      "w-3": { id: "w-3", minimized: false },
    },
    client: { focusOrder: ["w-2", "w-1", "w-3"] },
    desktops: [],
  } as unknown as OsDesktopRuntimeStore;
  const activateWindow = vi.fn();
  const openNewTab = vi.fn();
  return {
    manager,
    state,
    openPalette: vi.fn(),
    openPaletteSessions: vi.fn(),
    openNewSession: vi.fn(),
    toggleGlobalScope: vi.fn(),
    openCheatsheet: vi.fn(),
    openDesktops: vi.fn(),
    openWorkspaces: vi.fn(),
    cycleWorkspace: vi.fn(),
    cycleSession: vi.fn(),
    focusAttention: vi.fn(),
    toggleSidebar: vi.fn(),
    activateWindow,
    openNewTab,
  };
}

describe("dispatchWindowManagerAction — tab actions", () => {
  it("Should jump positionally, wrap next/previous from the active tab, and land last [UT-050]", () => {
    const frame = frameFixture();
    const jump = contextFixture(frame, "w-2");
    dispatchWindowManagerAction("window.tab.jump.3", jump);
    expect(jump.activateWindow).toHaveBeenCalledWith("w-3");

    const next = contextFixture(frame, "w-2");
    dispatchWindowManagerAction("window.tab.next", next);
    expect(next.activateWindow).toHaveBeenCalledWith("w-3");

    const wrap = contextFixture(frameFixture({ activeWindowId: "w-3" }), "w-3");
    dispatchWindowManagerAction("window.tab.next", wrap);
    expect(wrap.activateWindow).toHaveBeenCalledWith("w-1");

    const previous = contextFixture(frameFixture({ activeWindowId: "w-1" }), "w-1");
    dispatchWindowManagerAction("window.tab.previous", previous);
    expect(previous.activateWindow).toHaveBeenCalledWith("w-3");

    const last = contextFixture(frame, "w-2");
    dispatchWindowManagerAction("window.tab.last", last);
    expect(last.activateWindow).toHaveBeenCalledWith("w-3");
  });

  it("Should no-op jumps and cycling on a single-tab frame while ⌘T still opens the deck path [UT-052]", () => {
    const solo = frameFixture({
      id: "w-1",
      members: ["w-1"],
      activeWindowId: "w-1",
      stackId: null,
    });
    for (const actionId of [
      "window.tab.next",
      "window.tab.previous",
      "window.tab.last",
      "window.tab.jump.2",
    ] as const) {
      const context = contextFixture(solo, "w-1");
      dispatchWindowManagerAction(actionId, context);
      expect(context.activateWindow).not.toHaveBeenCalled();
    }

    const newTab = contextFixture(solo, "w-1");
    dispatchWindowManagerAction("window.tab.new", newTab);
    expect(newTab.openNewTab).toHaveBeenCalledWith("w-1");
  });

  it("Should open a standalone new tab when no window is focused [UT-082]", () => {
    const context = contextFixture(null, null);
    dispatchWindowManagerAction("window.tab.new", context);
    expect(context.openNewTab).toHaveBeenCalledWith(null);
  });

  it("Should dispatch reopen without requiring a focused window [UT-052]", () => {
    const context = contextFixture(null, null);
    dispatchWindowManagerAction("window.tab.reopen", context);
    expect(context.manager.reopenWindow).toHaveBeenCalledOnce();
  });

  it("Should ignore a jump slot beyond the frame's members", () => {
    const context = contextFixture(frameFixture(), "w-2");
    dispatchWindowManagerAction("window.tab.jump.8", context);
    expect(context.activateWindow).not.toHaveBeenCalled();
  });
});

describe("dispatchWindowManagerAction — navigation v2", () => {
  it("Should focus the previous MRU window and ignore minimized entries [UT-072]", () => {
    const context = contextFixture(null, "w-2");
    context.state = {
      ...context.state,
      client: { ...context.state.client!, focusOrder: ["w-2", "w-3", "w-1"] },
      windows: {
        ...context.state.windows,
        "w-3": { ...context.state.windows["w-3"]!, minimized: true },
      },
    };

    dispatchWindowManagerAction("window.focus.last", context);

    expect(context.activateWindow).toHaveBeenCalledExactlyOnceWith("w-1");
  });

  it("Should share stable desktop order with numeric switching and no-op missing slots [UT-073]", () => {
    const context = contextFixture(null, null);
    context.state.desktops = [
      { id: "desktop:c", order: 1 },
      { id: "desktop:b", order: 0 },
      { id: "desktop:a", order: 0 },
    ] as unknown as OsDesktopRuntimeStore["desktops"];

    dispatchWindowManagerAction("desktop.switch.2", context);
    expect(context.manager.switchDesktop).toHaveBeenCalledExactlyOnceWith("desktop:b");

    dispatchWindowManagerAction("desktop.switch.9", context);
    expect(context.manager.switchDesktop).toHaveBeenCalledOnce();
  });

  it("Should route session and attention actions through their shared shell owners [UT-074]", () => {
    const context = contextFixture(null, null);
    dispatchWindowManagerAction("session.cycle.next", context);
    dispatchWindowManagerAction("session.focus.attention", context);

    expect(context.cycleSession).toHaveBeenCalledExactlyOnceWith("next");
    expect(context.focusAttention).toHaveBeenCalledOnce();
  });
});
