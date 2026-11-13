import type {
  OsDesktopRuntimeStore,
  OsOpenTarget,
  OsWindowRoute,
  WindowManagerCommandOutcome,
  WindowManagerOpenOutcome,
} from "./os-types";
import type { RouteWriteId } from "./routing-write-echoes";
import type { FocusWindowOptions } from "./window-manager-types";

/**
 * The coordinator's history writes. Each carries the write id the new entry's history state must
 * hold under `ROUTE_WRITE_STATE_KEY`, so the entry's later report names the write that created it.
 */
export interface OsRouterPort {
  /** Pushes one history entry reflecting the location. */
  navigate(route: OsWindowRoute, writeId: RouteWriteId): void;
  /** Truths up hydration or remote authority without manufacturing history. */
  replace(route: OsWindowRoute, writeId: RouteWriteId): void;
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
}
