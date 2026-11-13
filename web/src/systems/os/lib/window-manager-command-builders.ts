import type { DropEdge } from "./drop-edge";
import type {
  MoveWindowInput,
  OsArrangePreset,
  OsCloseScope,
  OsNavigateMode,
  OsOpenTarget,
  OsWindowRoute,
} from "./os-types";
import type {
  DesktopId,
  LayoutTab,
  TabId,
  WindowId,
  WindowManagerCommandInput,
} from "./window-manager-types";
import { defaultOsWindowRoute } from "./window-manager-view";

/**
 * A new window opens as a tab after the active tab (the daemon's default policy), as a tab right
 * after `afterWindowId`'s tab, or with `placement: "split"` beside the focused window.
 */
export function openWindowCommand(
  target: OsOpenTarget,
  id: string,
  desktopId: string
): WindowManagerCommandInput {
  return {
    commandId: "window.open",
    payload: {
      window: {
        id,
        app: target.app,
        ...(target.instanceKey ? { instance_key: target.instanceKey } : {}),
        route: target.route ?? defaultOsWindowRoute(target.app),
        desktop_id: desktopId,
        ...(target.placement === "split" ? { insert_tiled: true } : {}),
        ...(target.afterWindowId ? { stack_target_window_id: target.afterWindowId } : {}),
      },
    },
  };
}

export function focusWindowCommand(windowId: string): WindowManagerCommandInput {
  return {
    commandId: "window.focus",
    payload: { window_id: windowId, direction: "" },
    rebase: { windowId },
  };
}

/** Moves the window's tab to `index`; the daemon keeps it inside its pinned or unpinned range. */
export function reorderTabCommand(windowId: string, index: number): WindowManagerCommandInput {
  return {
    commandId: "window.tab.reorder",
    payload: { window_id: windowId, index },
    rebase: { windowId },
  };
}

/** Makes the window's tab the active tab for this client and the desktop default. */
export function activateTabCommand(windowId: string): WindowManagerCommandInput {
  return {
    commandId: "window.tab.activate",
    payload: { window_id: windowId },
    rebase: { windowId },
  };
}

export function pinWindowCommand(windowId: string, pinned: boolean): WindowManagerCommandInput {
  return { commandId: "window.pin", payload: { window_id: windowId, pinned } };
}

export function reopenWindowCommand(): WindowManagerCommandInput {
  return { commandId: "window.reopen", payload: {} };
}

/**
 * `tab` is the wire default and travels as an absent scope; every other scope is one command,
 * one revision, one closed entry.
 */
export function closeWindowCommand(
  windowId: string,
  scope: OsCloseScope = "tab"
): WindowManagerCommandInput {
  return {
    commandId: "window.close",
    payload: { window_id: windowId, ...(scope === "tab" ? {} : { scope }) },
  };
}

/**
 * `pop` discards the payload route by contract — strict decode rejects a non-empty route
 * alongside it — so the builder never emits one.
 */
export function navigateWindowCommand(
  windowId: string,
  route: OsWindowRoute | null,
  mode: OsNavigateMode = "replace"
): WindowManagerCommandInput {
  if (mode === "pop") {
    return { commandId: "window.navigate", payload: { window_id: windowId, mode: "pop" } };
  }
  return {
    commandId: "window.navigate",
    payload: {
      window_id: windowId,
      route: route ?? { pathname: "/", search: {} },
      ...(mode === "push" ? { mode: "push" } : {}),
    },
  };
}

/** Replace navigation that also re-keys the window's app instance. */
export function retargetWindowCommand(
  windowId: string,
  instanceKey: string,
  route: OsWindowRoute
): WindowManagerCommandInput {
  return {
    commandId: "window.navigate",
    payload: { window_id: windowId, route, instance_key: instanceKey },
  };
}

/**
 * `tab` places the window (or its whole tab with `moveGroup`) as a tab after the target's tab, or
 * at the end of the destination bar; an edge placement splits beside the target window, or at the
 * root edge of `targetTabId`.
 */
export function moveWindowCommand(id: string, input: MoveWindowInput): WindowManagerCommandInput {
  return {
    commandId: "window.move",
    payload: {
      window_id: id,
      destination_desktop_id: input.destinationDesktopId,
      ...(input.targetWindowId ? { target_window_id: input.targetWindowId } : {}),
      ...(input.targetTabId ? { target_tab_id: input.targetTabId } : {}),
      placement: input.placement,
      move_group: input.moveGroup ?? false,
    },
  };
}

/** The window facts a split command needs: a single-window tab has nothing to unsplit or tile. */
type SplitMember = Readonly<{ id: WindowId; desktopId: DesktopId; tabId: TabId; inSplit: boolean }>;

/** "Move to new tab": a split member becomes its own tab right after its source tab (US-010). */
export function unsplitWindowCommand(window: SplitMember): WindowManagerCommandInput | null {
  if (!window.inSplit) return null;
  return {
    ...moveWindowCommand(window.id, { destinationDesktopId: window.desktopId, placement: "tab" }),
    rebase: { windowId: window.id, tabId: window.tabId },
  };
}

/**
 * `window.tile.left|right|top|bottom`: the window moves to that root edge of its own tab
 * (US-012.AC-2). A single-window tab has no edge to move to.
 */
export function tileWindowCommand(
  window: SplitMember,
  edge: DropEdge
): WindowManagerCommandInput | null {
  if (!window.inSplit) return null;
  return {
    ...moveWindowCommand(window.id, {
      destinationDesktopId: window.desktopId,
      targetTabId: window.tabId,
      placement: edge,
    }),
    rebase: { windowId: window.id, tabId: window.tabId },
  };
}

/**
 * "Split with ▸": the chosen tab joins `hostTabId` as a split, on the right (US-012.AC-3). The
 * chosen tab moves whole, so a split tab brings its members.
 */
export function splitWithCommand(
  desktopId: DesktopId,
  hostTabId: TabId,
  chosenLeadId: WindowId
): WindowManagerCommandInput {
  return {
    ...moveWindowCommand(chosenLeadId, {
      destinationDesktopId: desktopId,
      targetTabId: hostTabId,
      placement: "right",
      moveGroup: true,
    }),
    rebase: { windowId: chosenLeadId, tabId: hostTabId },
  };
}

/**
 * The bar index "Move left" / "Move right" reaches: one step within the tab's pinned or unpinned
 * range, or null at the range's edge, where the item is absent.
 */
export function adjacentTabIndex(
  tabs: readonly LayoutTab[],
  tabId: TabId,
  direction: "left" | "right"
): number | null {
  const index = tabs.findIndex(tab => tab.id === tabId);
  const tab = tabs[index];
  if (tab === undefined) return null;
  const pins = tabs.filter(candidate => candidate.pinned).length;
  const [first, last] = tab.pinned ? [0, pins - 1] : [pins, tabs.length - 1];
  const next = direction === "left" ? index - 1 : index + 1;
  return next < first || next > last ? null : next;
}

export function swapWindowsCommand(
  firstWindowId: string,
  secondWindowId: string
): WindowManagerCommandInput {
  return {
    commandId: "window.swap",
    payload: { first_window_id: firstWindowId, second_window_id: secondWindowId },
  };
}

const ARRANGEMENT_BY_PRESET = {
  "two-up": "horizontal",
  grid: "grid",
  "main-stack": "main_stack",
  columns: "horizontal",
} as const satisfies Record<OsArrangePreset, string>;

/** One `layout.arrange` gathers the participants, in order, into one split tab. */
export function arrangeLayoutCommand(
  desktopId: string,
  participants: readonly string[],
  preset: OsArrangePreset
): WindowManagerCommandInput | null {
  if (participants.length < 2) return null;
  return {
    commandId: "layout.arrange",
    payload: {
      desktop_id: desktopId,
      window_ids: [...participants],
      arrangement: ARRANGEMENT_BY_PRESET[preset],
    },
  };
}

export function resizeLayoutCommand(
  splitId: string,
  boundaryIndex: number,
  delta: number
): WindowManagerCommandInput {
  return {
    commandId: "layout.resize",
    payload: { split_id: splitId, boundary_index: boundaryIndex, delta },
    rebase: { splitId, boundaryIndex },
  };
}

/** Evens one split, or every split of a tab. */
export function balanceLayoutCommand(target: {
  tabId?: string;
  splitId?: string;
}): WindowManagerCommandInput | null {
  if (target.splitId) {
    return { commandId: "layout.balance", payload: { split_id: target.splitId } };
  }
  if (target.tabId) {
    return {
      commandId: "layout.balance",
      payload: { tab_id: target.tabId },
      rebase: { tabId: target.tabId },
    };
  }
  return null;
}
