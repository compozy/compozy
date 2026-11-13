import type {
  GuardedLocation,
  UnsavedWindowEditsPort,
  WindowNavigationGuardPort,
} from "@/hooks/use-window-navigation-guard";

import { sameAgentWindowContent } from "../apps/agents/agent-window-location";
import { resolveAppDescriptorForPath } from "./app-catalog";
import type { OsDesktopRuntimeStore, OsWindow, OsWindowRoute } from "./os-types";
import { mruWindowInstance } from "./window-instance-lookup";
import { sameOsWindowRoute } from "./window-manager-route";

type RouteState = Pick<OsDesktopRuntimeStore, "client" | "focusedId" | "windows">;
type GuardState = RouteState & Pick<OsDesktopRuntimeStore, "activeDesktopId">;

/**
 * Whether a router navigation to `next` would replace what window `windowId` shows, resolved
 * the way the routing coordinator lands it:
 * - a route outside the shell layout unmounts every window, so it replaces the content;
 * - a location naming what the window already shows (Back to the window's own route, a search
 *   that changes nothing it shows, a legacy path that resolves to the same page) replaces nothing;
 * - the focused window's route is the coordinator writing the URL after focus (a tab click,
 *   the dock, the palette): it lands in that window;
 * - any other location reconciles onto the most recently focused instance of the app that owns
 *   it (`decideRouteReconciliation`), or opens a new window when there is none.
 * A window the manager does not know answers `true`, so a guard there holds as before.
 */
export function routeReplacesWindowContent(
  state: RouteState,
  windowId: string,
  next: GuardedLocation
): boolean {
  const win = state.windows[windowId];
  if (win === undefined) return true;
  if (next.routeId !== undefined && !insideShell(next.routeId)) return true;
  const route = { pathname: next.pathname, search: next.search };
  if (sameWindowContent(win, route)) return false;
  const focused = state.focusedId === null ? undefined : state.windows[state.focusedId];
  if (focused !== undefined && sameOsWindowRoute(focused.route, route)) return false;
  const resolved = resolveAppDescriptorForPath(route.pathname);
  if (resolved === null) return false;
  const target = mruWindowInstance(state.windows, state.client?.focusOrder ?? [], {
    app: resolved.app.id,
    instanceKey: resolved.instanceKey,
  });
  return target?.id === windowId;
}

/** Routes under the `_app` layout render inside the shell; any other route replaces it. */
function insideShell(routeId: string): boolean {
  return routeId === SHELL_ROUTE_ID || routeId.startsWith(`${SHELL_ROUTE_ID}/`);
}

const SHELL_ROUTE_ID = "/_app";

/** Whether `route` shows what the window shows, as its app reads its own location. */
function sameWindowContent(win: OsWindow, route: OsWindowRoute): boolean {
  if (sameOsWindowRoute(win.route, route)) return true;
  return win.app === "agents" && sameAgentWindowContent(win.route, route);
}

/** The shell's answers to navigation guards, read from the window manager when they ask. */
export function createWindowNavigationGuard(
  manager: { getState(): GuardState },
  unsavedEdits: UnsavedWindowEditsPort
): WindowNavigationGuardPort {
  return {
    unsavedEdits,
    replacesWindowContent: (windowId, next) =>
      routeReplacesWindowContent(manager.getState(), windowId, next),
    isWindowShown: windowId => {
      const state = manager.getState();
      const win = state.windows[windowId];
      return win !== undefined && win.shown && win.desktopId === state.activeDesktopId;
    },
  };
}
