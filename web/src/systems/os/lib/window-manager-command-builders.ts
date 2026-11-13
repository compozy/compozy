import type {
  MoveWindowInput,
  OsArrangePreset,
  OsCloseScope,
  OsNavigateMode,
  OsOpenTarget,
  OsWindowRoute,
} from "./os-types";
import type { WindowManagerCommandInput } from "./window-manager-types";
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
