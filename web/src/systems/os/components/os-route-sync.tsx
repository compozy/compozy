import { useLocation } from "@tanstack/react-router";
import { useLayoutEffect } from "react";

import { useOsShell } from "../hooks/use-os-shell";
import type { OsAppId } from "../lib/os-types";

/**
 * Route leaves become sync-controllers (Routing Model rule 1): the component
 * reports the matched location to the coordinator as a route-originated cause
 * and renders null — windows render outside the router `Outlet`. The route
 * file keeps its path, `validateSearch`, `beforeLoad`, and `loader`.
 *
 * Each history entry reports once, with the key the router's history gave it: Back or Forward to
 * another entry with the same URL is a new report, and the key tells the coordinator's own
 * navigation from the operator's.
 */
export function createOsRouteSync(app: OsAppId) {
  function OsRouteSync() {
    const { coordinator } = useOsShell();
    const location = useLocation();
    // The router structurally shares `search`, so its identity only changes
    // when the parsed search actually changes.
    const pathname = location.pathname;
    const search = location.search as Record<string, unknown>;
    const entryKey = location.state.__TSR_key;
    useLayoutEffect(() => {
      coordinator.reportRouteMatch({ pathname, search }, entryKey ?? null);
    }, [coordinator, pathname, search, entryKey]);
    return null;
  }
  OsRouteSync.displayName = `OsRouteSync(${app})`;
  return OsRouteSync;
}
