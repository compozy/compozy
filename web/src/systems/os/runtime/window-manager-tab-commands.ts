import type {
  OsCloseScope,
  OsNavigateMode,
  OsOpenTarget,
  OsWindowRoute,
  WindowManagerCommandOutcome,
} from "../lib/os-types";
import {
  DEFAULT_WINDOW_MANAGER_FLOATING_RECT,
  defaultOsWindowRoute,
  normalizedRectToWire,
} from "../lib/window-manager-view";
import type { WindowManagerCommandInput } from "../lib/window-manager-types";
import { windowManagerStore } from "../stores/window-manager-store";
import { randomWindowManagerId, WindowManagerRuntimeCore } from "./window-manager-runtime-core";

/**
 * Builders for the tab half of the command surface. They live apart from
 * `window-manager-command-builders.ts` because that file owns the pre-tab
 * geometry commands and both are near the file-size cap.
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
        floating_rect: normalizedRectToWire(DEFAULT_WINDOW_MANAGER_FLOATING_RECT),
        insert_tiled: false,
        ...(target.stackTargetWindowId
          ? { stack_target_window_id: target.stackTargetWindowId }
          : {}),
      },
    },
  };
}

/** Bulk group: drag-merge, merge-all, and agent setup all reduce to one revision. */
export function groupWindowsCommand(
  targetWindowId: string,
  windowIds: readonly string[],
  insertIndex?: number
): WindowManagerCommandInput {
  return {
    commandId: "window.stack.group",
    payload: {
      target_window_id: targetWindowId,
      window_ids: [...windowIds],
      ...(insertIndex === undefined ? {} : { insert_index: insertIndex }),
    },
    rebase: { windowId: targetWindowId },
  };
}

export function reorderStackCommand(windowId: string, index: number): WindowManagerCommandInput {
  return {
    commandId: "window.stack.reorder",
    payload: { window_id: windowId, index },
    rebase: { windowId },
  };
}

/** Public noun is "activate"; `window.stack.set_active` stays the internal command ID. */
export function activateStackMemberCommand(windowId: string): WindowManagerCommandInput {
  return { commandId: "window.stack.set_active", payload: { window_id: windowId } };
}

export function pinWindowCommand(windowId: string, pinned: boolean): WindowManagerCommandInput {
  return { commandId: "window.pin", payload: { window_id: windowId, pinned } };
}

export function reopenWindowCommand(): WindowManagerCommandInput {
  return { commandId: "window.reopen", payload: {} };
}

/**
 * `tab` is the wire default and travels as an absent scope; every other scope is
 * one command, one revision, one closed entry.
 */
export function closeWindowCommand(
  windowId: string,
  scope: OsCloseScope = "tab"
): WindowManagerCommandInput {
  return {
    commandId: "window.close",
    payload: {
      window_id: windowId,
      minimize: false,
      ...(scope === "tab" ? {} : { scope }),
    },
  };
}

/**
 * `pop` discards the payload route by contract — strict decode rejects a
 * non-empty route alongside it — so the builder never emits one.
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

/**
 * Tab half of the controller. It sits between the transport core and the
 * geometry runtime so neither of those files carries the tab surface.
 */
export abstract class WindowManagerTabRuntime extends WindowManagerRuntimeCore {
  navigateWindow = (
    id: string,
    route: OsWindowRoute,
    mode: OsNavigateMode = "replace"
  ): WindowManagerCommandOutcome => {
    const outcome = this.dispatch(navigateWindowCommand(id, route, mode));
    if (!outcome.accepted) return outcome;
    // The daemon owns the route, but the URL and the deck label must not lag a
    // round trip behind the click that caused them.
    const intentId = randomWindowManagerId("wm-route");
    windowManagerStore.trigger.routeIntentSet({ intent: { id: intentId, windowId: id, route } });
    this.publish();
    return {
      accepted: true,
      completion: outcome.completion.then(applied => {
        windowManagerStore.trigger.routeIntentCleared({ windowId: id, intentId });
        this.publish();
        return applied;
      }),
    };
  };

  /** Breadcrumb / ⌘[ : the daemon resolves the destination from the nav stack. */
  popWindowRoute = (id: string): WindowManagerCommandOutcome => {
    const win = this.view.windows[id];
    if (!win || win.navStack.length === 0) {
      return { accepted: false, completion: Promise.resolve(false) };
    }
    return this.dispatch(navigateWindowCommand(id, null, "pop"));
  };

  groupWindows = (
    targetWindowId: string,
    windowIds: readonly string[],
    insertIndex?: number
  ): WindowManagerCommandOutcome => {
    const joiners = windowIds.filter(id => id !== targetWindowId);
    if (joiners.length === 0) return { accepted: false, completion: Promise.resolve(false) };
    return this.dispatch(groupWindowsCommand(targetWindowId, joiners, insertIndex));
  };

  reorderStackMember = (windowId: string, index: number): WindowManagerCommandOutcome =>
    this.dispatch(reorderStackCommand(windowId, index));

  activateStackMember = (windowId: string): WindowManagerCommandOutcome =>
    this.dispatch(activateStackMemberCommand(windowId));

  pinWindow = (windowId: string, pinned: boolean): WindowManagerCommandOutcome =>
    this.dispatch(pinWindowCommand(windowId, pinned));

  reopenWindow = (): WindowManagerCommandOutcome => this.dispatch(reopenWindowCommand());

  closeWindowScoped = (windowId: string, scope: OsCloseScope): Promise<boolean> =>
    this.dispatch(closeWindowCommand(windowId, scope)).completion;
}
