import type { OsWindowRoute } from "./os-types";
import type { RouteReconciliation } from "./routing-coordinator-contract";
import { sameOsWindowRoute } from "./window-manager-route";

const DESKTOP_DEFAULT_VIEW_INTENT_KEY = "_compozy_desktop_default";

export function isDesktopDefaultViewIntent(route: OsWindowRoute): boolean {
  const marker = route.search[DESKTOP_DEFAULT_VIEW_INTENT_KEY];
  return route.pathname === "/" && (marker === "1" || marker === 1);
}

export function withoutDesktopDefaultViewIntent(route: OsWindowRoute): OsWindowRoute {
  if (!isDesktopDefaultViewIntent(route)) return route;
  const { [DESKTOP_DEFAULT_VIEW_INTENT_KEY]: _intent, ...search } = route.search;
  return { pathname: route.pathname, search };
}

/**
 * Whether a queued route reconciliation carries the desktop default-view
 * intent: the route marks it, a pending intent reaches the desktop URL, or the
 * reconciliation it replaces already carried it for the same route.
 */
export function carriesDesktopDefaultViewIntent(
  route: OsWindowRoute,
  intentPending: boolean,
  current: RouteReconciliation | null
): boolean {
  const normalizedRoute = withoutDesktopDefaultViewIntent(route);
  return (
    isDesktopDefaultViewIntent(route) ||
    (intentPending && normalizedRoute.pathname === "/") ||
    (current?.desktopDefaultView === true &&
      sameOsWindowRoute(withoutDesktopDefaultViewIntent(current.route), normalizedRoute))
  );
}
