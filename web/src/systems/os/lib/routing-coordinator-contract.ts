import type {
  OsDesktopRuntimeStore,
  OsOpenTarget,
  OsWindowRoute,
  WindowManagerCommandOutcome,
  WindowManagerOpenOutcome,
} from "./os-types";

export interface OsRouterPort {
  /** Pushes one history entry reflecting the location. */
  navigate(route: OsWindowRoute): void;
  /** Truths up hydration or remote authority without manufacturing history. */
  replace(route: OsWindowRoute): void;
}

export type RoutingManager = {
  getState(): Pick<OsDesktopRuntimeStore, "client" | "windows" | "focusedId" | "hydration">;
  openOrFocus(target: OsOpenTarget): WindowManagerOpenOutcome;
  closeWindow(windowId: string): Promise<boolean>;
  focusWindow(windowId: string): WindowManagerCommandOutcome;
  restoreWindow(windowId: string): WindowManagerCommandOutcome;
  minimizeWindow(windowId: string): Promise<boolean>;
  zoomWindow(windowId: string): WindowManagerCommandOutcome;
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
  popWindowRoute(windowId: string): WindowManagerCommandOutcome;
};

export interface RouteReconciliation {
  token: number;
  route: OsWindowRoute;
  desktopDefaultView: boolean;
  inFlight: boolean;
}
