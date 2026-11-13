import { useLocation } from "@tanstack/react-router";
import { useLayoutEffect } from "react";

import { useOsShell } from "../hooks/use-os-shell";
import type { OsAppId } from "../lib/os-types";
import { routeWriteIdOf } from "../lib/routing-write-echoes";

/**
 * Route leaves become sync-controllers (Routing Model rule 1): the component
 * reports the matched location to the coordinator as a route-originated cause
 * and renders null — windows render outside the router `Outlet`. The route
 * file keeps its path, `validateSearch`, `beforeLoad`, and `loader`.
 *
 * Each history entry reports once, with the coordinator write its state carries: Back or Forward
 * to another entry with the same URL is a new report, and the write id tells the coordinator's
 * own write from operator navigation.
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
    const writeId = routeWriteIdOf(location.state);
    useLayoutEffect(() => {
      coordinator.reportRouteMatch({ pathname, search }, writeId);
    }, [coordinator, pathname, search, entryKey, writeId]);
    return null;
  }
  OsRouteSync.displayName = `OsRouteSync(${app})`;
  return OsRouteSync;
}
