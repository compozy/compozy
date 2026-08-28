import type { OsWindowRoute } from "./os-types";

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
