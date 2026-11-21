import type { QueryClient } from "@tanstack/react-query";

import type { SessionFolderPayload, SessionListFilters, SessionPayload } from "../types";
import { sessionCatalogOptions, sessionFoldersOptions } from "./session-catalog-options";
import {
  pendingSessionLayoutEdit,
  pendingSessionLayoutFilter,
  projectPartitionRows,
  SESSION_LIST_ROOT_FOLDER,
  sessionListPartitionFolders,
  sessionListPartitionOptions,
} from "./session-list-partitions";
import { visibleSessionOrder } from "./session-folders";
import { sessionListGroupFilters, type SessionListGroupSpec } from "./session-list-groups";
import { SESSION_LIST_PINNED_FOLD_KEY } from "./session-list-pinned";
import { sessionCatalogScopedFilters } from "./session-catalog-scope";
import type { SessionListSort } from "./session-list-preferences";
import type { SessionListShelf } from "./session-list-shelf";
import type { SessionFoldSet } from "./session-sidebar-scope";

/** What the list renders right now, as the session view model reports it. */
export interface SessionListOrderSource {
  catalog?: { sessions: readonly SessionPayload[] };
  layout?: { folders: readonly SessionFolderPayload[] };
  folderOf?: ReadonlyMap<string, string | null>;
  groups: readonly SessionListGroupSpec[];
  groupBy: string;
  groupFilters: SessionListFilters;
  sort: SessionListSort;
  collapsedGroupKeys: SessionFoldSet;
  shelf: SessionListShelf;
  search?: string;
  /** The Pinned section's rows, shown above everything else. */
  pinned?: { sessions: readonly SessionPayload[] };
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
    collapsedThreadIds: SessionFoldSet;
    collapsedFolderIds: SessionFoldSet;
  }
): SessionPayload[] {
  const grouped = view.groupBy !== "none";
  // A collapsed Pinned section hides its rows; a search opens it like every fold.
  const pinned = view.collapsedGroupKeys.has(SESSION_LIST_PINNED_FOLD_KEY)
    ? []
    : visibleSessionOrder(view.pinned?.sessions ?? [], {
        collapsedThreadIds: input.collapsedThreadIds,
        collapsedFolderIds: input.collapsedFolderIds,
        shelved: false,
        filtering: (view.search ?? "").trim() !== "",
      });
  return [...pinned, ...listedSessionOrder(view, input, grouped)];
}

function listedSessionOrder(
  view: SessionListOrderSource,
  input: Parameters<typeof sessionListVisibleOrder>[1],
  grouped: boolean
): SessionPayload[] {
  return visibleSessionOrder(view.catalog?.sessions ?? input.sessions, {
    collapsedThreadIds: input.collapsedThreadIds,
    collapsedFolderIds: input.collapsedFolderIds,
    folders: view.layout?.folders,
    folderOf: view.folderOf,
    shelved: view.shelf !== "active",
    filtering: (view.search ?? "").trim() !== "",
    collapsedGroupKeys: view.collapsedGroupKeys,
    groups: grouped ? view.groups.map(group => readGroupPages(view, group, input)) : undefined,
  });
}

function readGroupPages(
  view: SessionListOrderSource,
  group: SessionListGroupSpec,
  input: Parameters<typeof sessionListVisibleOrder>[1]
) {
  if (group.total === 0) return { key: group.key, sessions: [] };
  const { queryClient } = input;
  const pair = group.pair;
  const folders = pair ? queryClient.getQueryData(sessionFoldersOptions(pair).queryKey) : undefined;
  const edits = pair
    ? queryClient
        .getMutationCache()
        .findAll(pendingSessionLayoutFilter(pair.workspaceId, pair.profile))
        .flatMap(write => pendingSessionLayoutEdit(write.state.variables))
    : [];
  const readFolders = sessionListPartitionFolders(folders, edits);
  const filters = sessionCatalogScopedFilters(
    group.scope,
    sessionListGroupFilters(view, group, folders !== undefined),
    group.lens
  );
  if (!readFolders?.length) {
    const page = input.queryClient.getQueryData(sessionCatalogOptions(filters).queryKey)?.pages[0];
    return { key: group.key, folders, sessions: page?.sessions ?? [] };
  }
  const filtering = Boolean(filters.q?.trim());
  const folderIds = readFolders.flatMap(folder =>
    filtering || !input.collapsedFolderIds.has(folder.id) ? [folder.id] : []
  );
  const entries = [SESSION_LIST_ROOT_FOLDER, ...folderIds].map(folder => {
    const options = sessionListPartitionOptions(filters, folder, false);
    const page = input.queryClient.getQueryData(options.queryKey)?.pages[0];
    return [folder, { sessions: page?.sessions ?? [] }] as const;
  });
  return {
    key: group.key,
    folders,
    ...projectPartitionRows(entries, edits, filters.sort === "manual"),
  };
}
