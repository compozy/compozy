import { useLocation } from "@tanstack/react-router";
import { useLayoutEffect } from "react";

import { useOsShell } from "../hooks/use-os-shell";
import type { OsAppId } from "../lib/os-types";

/**
 * Route leaves become sync-controllers (Routing Model rule 1): the component
 * reports the matched location to the coordinator as a route-originated cause
 * and renders null — windows render outside the router `Outlet`. The route
 * file keeps its path, `validateSearch`, `beforeLoad`, and `loader`.
 */
export function createOsRouteSync(app: OsAppId) {
  function OsRouteSync() {
    const { coordinator } = useOsShell();
    const location = useLocation();
    // The router structurally shares `search`, so its identity only changes
    // when the parsed search actually changes.
    const pathname = location.pathname;
    const search = location.search as Record<string, unknown>;
    useLayoutEffect(() => {
      coordinator.reportRouteMatch({ pathname, search });
    }, [coordinator, pathname, search]);
    return null;
  }
  OsRouteSync.displayName = `OsRouteSync(${app})`;
  return OsRouteSync;
}
