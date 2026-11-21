import { sessionCatalogOptions } from "./session-catalog-options";
import type { SessionListFilters, SessionPayload } from "../types";

export const SESSION_LIST_ROOT_FOLDER = "@root";

export function sessionListPartitionOptions(
  filters: SessionListFilters,
  folder: string,
  countOnly: boolean
) {
  return sessionCatalogOptions({
    ...filters,
    folder,
    skip_total: false,
    ...(countOnly ? { limit: 1, include_health: false } : {}),
  });
}

export function partitionSessionRows<T extends { sessions: readonly SessionPayload[] }>(
  entries: Iterable<readonly [string, T]>
) {
  const sessions = new Map<string, SessionPayload>();
  const folderOf = new Map<string, string | null>();
  for (const [folder, page] of entries) {
    for (const session of page.sessions) {
      const container = folder === SESSION_LIST_ROOT_FOLDER ? null : folder;
      if (sessions.has(session.id) && (session.folder_id || null) !== container) continue;
      sessions.set(session.id, session);
      folderOf.set(session.id, container);
    }
  }
  return { sessions: [...sessions.values()], folderOf };
}
