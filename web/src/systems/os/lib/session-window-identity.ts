import { isRemoteComputerId, normalizeComputerId, type ComputerId } from "@/lib/computer-scope";

import type { OsWindowRoute } from "./os-types";

export function sessionWindowRouteComputer(route: OsWindowRoute | undefined): ComputerId {
  const computer = route?.search?.computer;
  return normalizeComputerId(typeof computer === "string" ? computer : undefined);
}

export function sessionWindowInstanceKey(sessionId: string, computerId: ComputerId): string {
  if (!isRemoteComputerId(computerId) || sessionId.length === 0) return sessionId;
  const prefix = `${computerId}:`;
  return sessionId.startsWith(prefix) ? sessionId : `${prefix}${sessionId}`;
}
