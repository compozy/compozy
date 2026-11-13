import type {
  OsDesktopRuntimeStore,
  OsOpenTarget,
  OsWindowRoute,
  WindowManagerCommandOutcome,
  WindowManagerOpenOutcome,
} from "./os-types";
import type { FocusWindowOptions } from "./window-manager-types";

/**
 * The coordinator's history writes. They add no state to the entry; the router bridge reports the
 * entry each write created to the coordinator's `RoutingEntryLedger`.
 *
 * A write is a projection of authoritative window-manager state, which window content renders
 * from: a navigation blocker cannot hold it back (the content already changed), so it never
 * answers one and commits inside the call. Only an operator navigation the coordinator writes for
 * a control (`origin: "operator"`) answers blockers, as any router navigation does.
 */
export interface OsRouterPort {
  /** Pushes one history entry reflecting the location. */
  navigate(route: OsWindowRoute, origin?: "operator"): void;
  /** Truths up hydration or remote authority without manufacturing history. */
  replace(route: OsWindowRoute): void;
}

export type RoutingManager = {
  getState(): Pick<OsDesktopRuntimeStore, "client" | "windows" | "focusedId" | "hydration">;
  openOrFocus(target: OsOpenTarget): WindowManagerOpenOutcome;
  /** `viewOnly` takes the window off the desk without the operator guard; its item keeps running. */
  closeWindow(windowId: string, viewOnly?: boolean): Promise<boolean>;
  focusWindow(windowId: string, options?: FocusWindowOptions): WindowManagerCommandOutcome;
  navigateWindow(
    windowId: string,
    route: OsWindowRoute,
    mode?: "replace" | "push" | "pop"
  ): WindowManagerCommandOutcome;
  retargetWindow(
    windowId: string,
    instanceKey: string,
    route: OsWindowRoute
  ): WindowManagerCommandOutcome;
  popWindowRoute(windowId: string, expectedRoute?: OsWindowRoute): WindowManagerCommandOutcome;
};

export interface RouteReconciliation {
  token: number;
  route: OsWindowRoute;
  desktopDefaultView: boolean;
  inFlight: boolean;
  /** The route lands a just-created session; retries keep preferring a Session window. */
  createdSession?: boolean;
  /** This client's own navigation asked for it: the keyboard follows the focus it lands (#1655). */
  keyboard?: boolean;
}
