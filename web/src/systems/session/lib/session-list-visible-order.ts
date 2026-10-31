import type { QueryClient } from "@tanstack/react-query";

import type { SessionFolderPayload, SessionListFilters, SessionPayload } from "../types";
import { sessionCatalogOptions, sessionFoldersOptions } from "./session-catalog-options";
import { visibleSessionOrder } from "./session-folders";
import { sessionListGroupFilters, type SessionListGroupSpec } from "./session-list-groups";
import { sessionCatalogScopedFilters } from "./session-catalog-scope";
import type { SessionListSort } from "./session-list-preferences";
import type { SessionListShelf } from "./session-list-shelf";

/** What the list renders right now, as the session view model reports it. */
export interface SessionListOrderSource {
  catalog?: { sessions: readonly SessionPayload[] };
  layout?: { folders: readonly SessionFolderPayload[] };
  groups: readonly SessionListGroupSpec[];
  groupBy: string;
  groupFilters: SessionListFilters;
  sort: SessionListSort;
  collapsedGroupKeys: ReadonlySet<string>;
  shelf: SessionListShelf;
  search?: string;
}

/**
 * The session rows a keyboard cycle may land on, in on-screen order. Each
 * expanded group's current page and folders are read from the query cache at
 * invocation — the exact queries the group bodies own — rather than loading
 * another catalog for keyboard navigation.
 */
export function sessionListVisibleOrder(
  view: SessionListOrderSource,
  input: {
    queryClient: QueryClient;
    /** The page the flat list shows (falls back to the attention modal's page). */
    sessions: readonly SessionPayload[];
    collapsedThreadIds: ReadonlySet<string>;
    collapsedFolderIds: ReadonlySet<string>;
  }
): SessionPayload[] {
  const grouped = view.groupBy !== "none";
  return visibleSessionOrder(view.catalog?.sessions ?? input.sessions, {
    collapsedThreadIds: input.collapsedThreadIds,
    collapsedFolderIds: input.collapsedFolderIds,
    folders: view.layout?.folders,
    shelved: view.shelf !== "active",
    filtering: (view.search ?? "").trim() !== "",
    collapsedGroupKeys: view.collapsedGroupKeys,
    groups: grouped
      ? view.groups.map(group => {
          const filters = sessionCatalogScopedFilters(
            group.scope,
            sessionListGroupFilters(view, group),
            group.lens
          );
          return {
            key: group.key,
            sessions:
              input.queryClient.getQueryData(sessionCatalogOptions(filters).queryKey)?.pages[0]
                ?.sessions ?? [],
            folders: group.pair
              ? input.queryClient.getQueryData(sessionFoldersOptions(group.pair).queryKey)
              : undefined,
          };
        })
      : undefined,
  });
}
