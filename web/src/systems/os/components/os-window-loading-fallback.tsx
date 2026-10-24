import type { ComponentType } from "react";

import { useDesktop } from "../hooks/use-desktop";
import type { OsWindowRoute } from "../lib/os-types";

/**
 * Hosts an app's loading frame at its window's current route. The route is read here, inside the
 * fallback's own subtree, so the window member keeps exactly the subscriptions it has: only a
 * window that is loading pays for this one, and only while it loads.
 */
export function OsWindowLoadingFallback({
  Fallback,
  windowId,
}: {
  Fallback: ComponentType<{ route: OsWindowRoute }>;
  windowId: string;
}) {
  const route = useDesktop(state => state.windows[windowId]?.route);
  return route ? <Fallback route={route} /> : null;
}
