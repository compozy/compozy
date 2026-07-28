import type { SessionListFilters } from "@/systems/session";

/** Newest active sessions the working-now surface shows. */
export const WORKING_NOW_SESSION_LIMIT = 6;

/**
 * The exact active-session filter (minus workspace) the working-now surface
 * reads. The hook and the route preload both build their sessions query from
 * this so the warmed cache entry and the first-paint query normalize to one
 * key — workspace is applied per call site (hook via `useSessions`'s first arg,
 * preload via a spread). Drift here is what let the preload warm `limit: 1`
 * while the hook read `limit: 6`, so a mounted dashboard refetched instead of
 * reusing the warmed page.
 */
export function homeWorkingNowSessionFilters(): Omit<SessionListFilters, "workspace"> {
  return {
    state: "active",
    type: "user",
    sort: "last_activity",
    limit: WORKING_NOW_SESSION_LIMIT,
  };
}
