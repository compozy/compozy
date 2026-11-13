import { runViewTransition } from "@compozy/ui";

import type {
  OsCloseScope,
  OsNavigateMode,
  OsWindowRoute,
  WindowManagerCommandFence,
  WindowManagerCommandOutcome,
} from "../lib/os-types";
import {
  activateTabCommand,
  closeWindowCommand,
  navigateWindowCommand,
  pinWindowCommand,
  reopenWindowCommand,
  reorderTabCommand,
  retargetWindowCommand,
} from "../lib/window-manager-command-builders";
import { windowManagerStore } from "../stores/window-manager-store";
import { WindowManagerRuntimeCore } from "./window-manager-runtime-core";
import { randomWindowManagerId } from "./window-manager-runtime-helpers";
import { windowCloseTargets, type WindowCloseGuard } from "../lib/window-close-targets";
import { windowManagerCommandsAvailable } from "../lib/window-manager-command-availability";
import { sameOsWindowRoute } from "../lib/window-manager-route";
import { isOsReducedMotion } from "../lib/reduced-motion";

/**
 * Tab half of the controller: navigation, the tab bar's commands, and guarded
 * closes. It sits between the transport core and the layout runtime.
 */
export abstract class WindowManagerTabRuntime extends WindowManagerRuntimeCore {
  private closeGuard: WindowCloseGuard | null = null;
  private closePending = false;

  /** Installs operator lifecycle admission; replacing it invalidates pending close requests. */
  setCloseGuard(guard: WindowCloseGuard | null): void {
    this.closeGuard = guard;
  }
  navigateWindow = (
    id: string,
    route: OsWindowRoute,
    mode: OsNavigateMode = "replace"
  ): WindowManagerCommandOutcome => {
    const outcome = this.dispatch(navigateWindowCommand(id, route, mode));
    if (!outcome.accepted) return outcome;
    return this.withRouteIntent(id, route, outcome, mode === "push" ? "drill-in" : null);
  };

  /**
   * The daemon owns the route, but the URL and the tab label must not lag a
   * round trip behind the click that caused them: publish the route as an
   * intent until the command settles.
   */
  private withRouteIntent(
    id: string,
    route: OsWindowRoute,
    outcome: WindowManagerCommandOutcome,
    transition: "drill-in" | "drill-out" | null
  ): WindowManagerCommandOutcome {
    const intentId = randomWindowManagerId("wm-route");
    let settled = false;
    const applyIntent = () => {
      // A transition's update runs a frame later; never resurrect a settled intent.
      if (settled) return;
      windowManagerStore.trigger.routeIntentSet({ intent: { id: intentId, windowId: id, route } });
      this.publish();
    };
    if (transition && this.animatesInWindow(id)) {
      void runViewTransition(applyIntent, { types: [transition] });
    } else {
      applyIntent();
    }
    return {
      accepted: true,
      completion: outcome.completion.then(applied => {
        settled = true;
        windowManagerStore.trigger.routeIntentCleared({ windowId: id, intentId });
        this.publish();
        return applied;
      }),
    };
  }

  /**
   * In-window drill only: a push/pop inside the focused, on-screen window with
   * motion allowed. Focus changes, remote routes, and everything else stay
   * un-animated.
   */
  private animatesInWindow(id: string): boolean {
    const view = this.view;
    const win = view.windows[id];
    if (!win?.shown || view.focusedId !== id) return false;
    return !isOsReducedMotion(view);
  }

  /**
   * In-place instance switch: the window keeps its place while the daemon
   * re-keys it to another instance and resets its nav stack.
   */
  retargetWindow = (
    id: string,
    instanceKey: string,
    route: OsWindowRoute
  ): WindowManagerCommandOutcome => {
    const outcome = this.dispatch(retargetWindowCommand(id, instanceKey, route));
    if (!outcome.accepted) return outcome;
    const intentId = randomWindowManagerId("wm-route");
    windowManagerStore.trigger.routeIntentSet({
      intent: { id: intentId, windowId: id, route, instanceKey: instanceKey || null },
    });
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

  /**
   * Breadcrumb / ⌘[ : the daemon resolves the destination from the nav stack.
   * When the caller already knows the destination and it matches the stack
   * top, it renders optimistically so the drill-out can animate like a push.
   */
  popWindowRoute = (id: string, expectedRoute?: OsWindowRoute): WindowManagerCommandOutcome => {
    const win = this.view.windows[id];
    if (!win || win.navStack.length === 0) {
      return { accepted: false, completion: Promise.resolve(false) };
    }
    const outcome = this.dispatch(navigateWindowCommand(id, null, "pop"));
    const top = win.navStack[win.navStack.length - 1];
    if (!outcome.accepted || !expectedRoute || !top || !sameOsWindowRoute(top, expectedRoute)) {
      return outcome;
    }
    return this.withRouteIntent(id, expectedRoute, outcome, "drill-out");
  };

  reorderTab = (
    windowId: string,
    index: number,
    fence?: WindowManagerCommandFence
  ): WindowManagerCommandOutcome =>
    this.dispatch({ ...reorderTabCommand(windowId, index), ...fence });

  activateTab = (windowId: string): WindowManagerCommandOutcome =>
    this.dispatch(activateTabCommand(windowId));

  pinWindow = (windowId: string, pinned: boolean): WindowManagerCommandOutcome =>
    this.dispatch(pinWindowCommand(windowId, pinned));

  reopenWindow = (): WindowManagerCommandOutcome => this.dispatch(reopenWindowCommand());

  /**
   * Removes one window from the desk without the operator's close guard, so what it shows keeps
   * running: a terminal window leaves the desk and its terminal is not terminated. Refused while
   * an operator close is being confirmed, so it never races that close's fenced command. Fenced to
   * the layout revision the caller inspected: a command queued earlier that repurposes the window
   * moves the revision, so the daemon refuses this close instead of closing what it shows now.
   */
  closeWindowViewOnly = async (windowId: string): Promise<boolean> => {
    if (this.closePending || !windowManagerCommandsAvailable(this.view)) return false;
    return this.dispatch({
      ...closeWindowCommand(windowId),
      expectedRevision: this.view.snapshot?.revision,
    }).completion;
  };

  /** Serializes close admission and fences the final command to the inspected targets and revision. */
  closeWindowScoped = async (windowId: string, scope: OsCloseScope): Promise<boolean> => {
    if (!this.closeGuard) return this.dispatch(closeWindowCommand(windowId, scope)).completion;
    if (this.closePending || !windowManagerCommandsAvailable(this.view)) return false;
    const binding = this.binding;
    const guard = this.closeGuard;
    const targets = windowCloseTargets(this.view, windowId, scope);
    if (targets.length === 0) return false;
    const identity = (windows: typeof targets) =>
      JSON.stringify(
        windows.map(window => [window.id, window.app, window.instanceKey, window.route])
      );
    const captured = identity(targets);
    const isCurrent = () =>
      this.binding === binding &&
      this.closeGuard === guard &&
      windowManagerCommandsAvailable(this.view) &&
      identity(windowCloseTargets(this.view, windowId, scope)) === captured;
    this.closePending = true;
    try {
      if (!(await guard(targets, isCurrent)) || !isCurrent()) return false;
      // Never let a queued close expand to newly opened, unconfirmed tabs.
      return await this.dispatch({
        ...closeWindowCommand(windowId, scope),
        expectedRevision: this.view.snapshot?.revision,
      }).completion;
    } finally {
      this.closePending = false;
    }
  };
}
