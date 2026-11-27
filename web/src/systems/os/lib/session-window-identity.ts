import { isRemoteComputerId, normalizeComputerId, type ComputerId } from "@/lib/computer-scope";

import type { OsWindow, OsWindowRoute } from "./os-types";

export function sessionWindowRouteComputer(route: OsWindowRoute | undefined): ComputerId {
  const computer = route?.search?.computer;
  return normalizeComputerId(typeof computer === "string" ? computer : undefined);
}

export function sessionWindowInstanceKey(sessionId: string, computerId: ComputerId): string {
  if (!isRemoteComputerId(computerId) || sessionId.length === 0) return sessionId;
  const prefix = `${computerId}:`;
  return sessionId.startsWith(prefix) ? sessionId : `${prefix}${sessionId}`;
}

/** Includes the route owner while a saved window still uses its previous instance key. */
export function sessionWindowLookupKey(window: Pick<OsWindow, "instanceKey" | "route">): string {
  return sessionWindowInstanceKey(
    window.instanceKey ?? "",
    sessionWindowRouteComputer(window.route)
  );
}
