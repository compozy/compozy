// Suite: tab and window-head drag store
// Invariant: a drag only previews until the drop — a tab dragged inside the bar reorders within
// its pinned or unpinned range and activates nothing; past the bar it splits into the active tab
// (or the previously active one when it is itself active); a head held still over a tab for 400ms
// previews that tab; cancel restores everything and emits no command; a drop emits exactly one
// command fenced to the layout the drag started from (UT-034–UT-036).
// Boundary IN: named store events with resolved pointer zones, the dwell timer, emitted intents.
// Boundary OUT: DOM hit testing, the window-manager runtime and the daemon.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { HEAD_DWELL_MS } from "../../lib/drop-edge";
import {
  createOsTabDragStore,
  type TabDragIntent,
  type TabDragSession,
} from "../os-tab-drag-store";

const TAB_ORDER = ["tab:pin", "tab:a", "tab:b", "tab:c"];

function tabSession(tabId: string, overrides: Partial<TabDragSession> = {}): TabDragSession {
  return {
    desktopId: "desktop:1",
    revision: 7,
    source: { kind: "tab", tabId, windowId: `win:${tabId}`, nodeId: `leaf:${tabId}` },
    tabOrder: TAB_ORDER,
    pinnedCount: 1,
    activeTabId: "tab:a",
    splitTabId: tabId === "tab:a" ? "tab:c" : "tab:a",
    ...overrides,
  };
}

function headSession(): TabDragSession {
  return {
    ...tabSession("tab:a"),
    source: { kind: "head", tabId: "tab:a", windowId: "win:a1", nodeId: "leaf:a1" },
    splitTabId: "tab:a",
  };
}

function setup(session: TabDragSession) {
  const store = createOsTabDragStore();
  const intents: TabDragIntent[] = [];
  store.on("dropped", ({ intent }) => intents.push(intent));
  store.trigger.pressed({ session, point: { x: 100, y: 20 } });
  const context = () => store.getSnapshot().context;
  return { store, intents, context };
}

describe("tab drag store", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("keeps a press under the drag threshold a click", () => {
    const { store, intents, context } = setup(tabSession("tab:b"));
    store.trigger.moved({
      point: { x: 102, y: 21 },
      zone: { kind: "bar", index: 1, overTabId: "tab:a" },
      swapHeld: false,
    });
    expect(context().phase).toBe("pressed");
    store.trigger.released();
    expect(context().phase).toBe("idle");
    expect(intents).toEqual([]);
  });

  it("UT-034: a tab dragged inside the bar previews a reorder and activates nothing", () => {
    const { store, intents, context } = setup(tabSession("tab:c"));
    store.trigger.moved({
      point: { x: 40, y: 20 },
      zone: { kind: "bar", index: 1, overTabId: "tab:a" },
      swapHeld: false,
    });
    expect(context()).toMatchObject({
      phase: "dragging",
      mode: "reorder",
      insertionIndex: 1,
      contentTabId: "tab:a",
      target: { kind: "reorder", index: 1 },
    });
    // A regular tab cannot land among pinned tabs: the marker stops at the first regular slot.
    store.trigger.moved({
      point: { x: 5, y: 20 },
      zone: { kind: "bar", index: 0, overTabId: "tab:pin" },
      swapHeld: false,
    });
    expect(context().insertionIndex).toBe(1);
    expect(intents).toEqual([]);

    store.trigger.released();
    expect(intents).toEqual([
      {
        kind: "reorder",
        windowId: "win:tab:c",
        index: 1,
        fence: { expectedRevision: 7, rebase: { windowId: "win:tab:c" } },
      },
    ]);
    // The committed order holds until the daemon answers, then the bar follows the daemon.
    expect(context().settling).toEqual({
      desktopId: "desktop:1",
      order: ["tab:pin", "tab:c", "tab:a", "tab:b"],
    });
    store.trigger.settled();
    expect(context().settling).toBeNull();
  });

  it("UT-034: a pinned tab stays within the pinned range and a drop in place sends nothing", () => {
    const { store, intents, context } = setup(tabSession("tab:pin"));
    store.trigger.moved({
      point: { x: 300, y: 20 },
      zone: { kind: "bar", index: 4, overTabId: null },
      swapHeld: false,
    });
    expect(context().insertionIndex).toBe(1);
    expect(context().target).toEqual({ kind: "reorder", index: 0 });
    store.trigger.released();
    expect(intents).toEqual([]);
  });

  it("UT-035: past the bar a tab splits into the active tab's content", () => {
    const { store, intents, context } = setup(tabSession("tab:b"));
    store.trigger.moved({
      point: { x: 600, y: 400 },
      zone: {
        kind: "content",
        drop: { kind: "window", windowId: "win:tab:a", nodeId: "leaf:tab:a", edge: "right" },
      },
      swapHeld: true,
    });
    // The swap modifier is ignored across tabs.
    expect(context()).toMatchObject({
      mode: "split",
      contentTabId: "tab:a",
      target: { kind: "window", tabId: "tab:a", windowId: "win:tab:a", edge: "right" },
    });
    store.trigger.released();
    expect(intents).toEqual([
      {
        kind: "move",
        windowId: "win:tab:b",
        input: {
          destinationDesktopId: "desktop:1",
          placement: "right",
          moveGroup: true,
          targetWindowId: "win:tab:a",
        },
        fence: {
          expectedRevision: 7,
          rebase: { windowId: "win:tab:b", sourceNodeId: "leaf:tab:b", targetNodeId: "leaf:tab:a" },
        },
      },
    ]);
  });

  it("UT-035: dragging the active tab previews the previously active tab; cancel restores and sends nothing", () => {
    const { store, intents, context } = setup(tabSession("tab:a"));
    store.trigger.moved({
      point: { x: 600, y: 400 },
      zone: { kind: "content", drop: { kind: "root", edge: "left" } },
      swapHeld: false,
    });
    expect(context()).toMatchObject({
      contentTabId: "tab:c",
      target: { kind: "root", tabId: "tab:c", edge: "left" },
    });
    store.trigger.cancelled();
    expect(context()).toMatchObject({
      phase: "idle",
      session: null,
      contentTabId: null,
      target: null,
      settling: null,
    });
    expect(intents).toEqual([]);
  });

  it("UT-035: with no other tab there is no drop zone, and releasing outside changes nothing", () => {
    const { store, intents, context } = setup(tabSession("tab:a", { splitTabId: null }));
    store.trigger.moved({
      point: { x: 600, y: 400 },
      zone: { kind: "content", drop: { kind: "root", edge: "left" } },
      swapHeld: false,
    });
    expect(context().target).toBeNull();
    store.trigger.moved({ point: { x: 2000, y: 400 }, zone: { kind: "outside" }, swapHeld: false });
    store.trigger.released();
    expect(intents).toEqual([]);
  });

  it("UT-036: a head held still over a tab for 400ms previews that tab; movement resets the dwell", () => {
    const { store, context } = setup(headSession());
    const overB = (x: number) =>
      store.trigger.moved({
        point: { x, y: 20 },
        zone: { kind: "bar", index: 2, overTabId: "tab:b" },
        swapHeld: false,
      });
    overB(200);
    expect(context().dwell?.tabId).toBe("tab:b");
    vi.advanceTimersByTime(HEAD_DWELL_MS - 100);
    // Moving beyond the slop restarts the hold.
    overB(220);
    vi.advanceTimersByTime(HEAD_DWELL_MS - 100);
    expect(context().contentTabId).toBe("tab:a");
    vi.advanceTimersByTime(100);
    expect(context().contentTabId).toBe("tab:b");
    expect(context().dwell).toBeNull();
    // No insertion marker for a head in this slice: dropping on the bar sends nothing.
    expect(context().insertionIndex).toBeNull();
    expect(context().target).toBeNull();
  });

  it("UT-036: leaving the tab before the dwell ends previews nothing", () => {
    const { store, context } = setup(headSession());
    store.trigger.moved({
      point: { x: 200, y: 20 },
      zone: { kind: "bar", index: 2, overTabId: "tab:b" },
      swapHeld: false,
    });
    store.trigger.moved({
      point: { x: 600, y: 400 },
      zone: { kind: "content", drop: null },
      swapHeld: false,
    });
    vi.advanceTimersByTime(HEAD_DWELL_MS * 2);
    expect(context().contentTabId).toBe("tab:a");
  });

  it("drops a head beside a sibling, swaps under the modifier, and ignores a drop on itself", () => {
    const { store, intents, context } = setup(headSession());
    const over = (windowId: string, swapHeld: boolean) =>
      store.trigger.moved({
        point: { x: 600, y: 400 },
        zone: {
          kind: "content",
          drop: { kind: "window", windowId, nodeId: `leaf:${windowId}`, edge: "top" },
        },
        swapHeld,
      });
    over("win:a1", false);
    expect(context().target).toBeNull();
    over("win:a2", true);
    expect(context().target).toEqual({ kind: "swap", windowId: "win:a2", nodeId: "leaf:win:a2" });
    store.trigger.released();
    expect(intents).toEqual([
      {
        kind: "swap",
        firstWindowId: "win:a1",
        secondWindowId: "win:a2",
        fence: {
          expectedRevision: 7,
          rebase: { windowId: "win:a1", sourceNodeId: "leaf:a1", targetNodeId: "leaf:win:a2" },
        },
      },
    ]);
  });
});
