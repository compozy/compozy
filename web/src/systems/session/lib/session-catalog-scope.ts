import type { ProfileScopeParams } from "@/systems/profiles";
import { breadthQuery, sessionCatalogBreadthParams, type DataScope } from "@/systems/workspace";

import type { SessionListFilters } from "../types";

// Kept apart from `session-list-query.ts`: adapters import that module, and a
// workspace-barrel edge from it would close an import cycle through the session barrel.
/**
 * The full catalog query for a data scope and profile lens: the scope's breadth
 * spelling (`all_workspaces` or one `workspace_id`, `@user` for User) plus the
 * lens. A `null` scope reads as the aggregate key but is never enabled.
 */
export function sessionCatalogScopedFilters(
  scope: DataScope | null,
  filters: SessionListFilters,
  lens: ProfileScopeParams
): SessionListFilters {
  return {
    ...filters,
    ...sessionCatalogBreadthParams(breadthQuery(scope ?? { kind: "all" })),
    ...lens,
  };
}
