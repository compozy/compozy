import type { SessionListFilters } from "@/systems/session";

/** Newest working or waiting sessions the dashboard shows. */
export const WORKING_NOW_SESSION_LIMIT = 6;

/**
 * The session catalog filters by runtime badge before the page limit. The hook
 * and route preload use the same filter so they share the cached page.
 */
export function homeWorkingNowSessionFilters(): Omit<
  SessionListFilters,
  "workspace_id" | "all_workspaces"
> {
  return {
    state: "active",
    badge: "running,waiting",
    type: "user",
    sort: "last_activity",
    limit: WORKING_NOW_SESSION_LIMIT,
  };
}
