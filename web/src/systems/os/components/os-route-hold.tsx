import { useLocation } from "@tanstack/react-router";
import { useLayoutEffect } from "react";

import { useOsShell } from "../hooks/use-os-shell";

/**
 * Routed decision surfaces render this instead of a route sync (Routing Model rule 1). The
 * location intentionally opens no window — the cross-workspace deep-link confirmation asks
 * before anything foreign is opened — so the coordinator holds the URL until the decision
 * resolves instead of truing it up to the focused window.
 */
export function OsRouteHold() {
  const { coordinator } = useOsShell();
  const location = useLocation();
  const pathname = location.pathname;
  const search = location.search as Record<string, unknown>;
  useLayoutEffect(() => {
    coordinator.holdRoute({ pathname, search });
    return () => coordinator.releaseRouteHold();
  }, [coordinator, pathname, search]);
  return null;
}
