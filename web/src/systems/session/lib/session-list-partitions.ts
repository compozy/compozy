import { sessionCatalogOptions } from "./session-catalog-options";
import { sessionFolderId } from "./session-folders";
import type { SessionFolderPayload, SessionListFilters, SessionPayload } from "../types";

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

export interface PendingSessionLayoutEdit {
  update: (sessions: readonly SessionPayload[], manual: boolean) => SessionPayload[];
  removedFolder?: SessionFolderPayload;
}

export function sessionLayoutMutationKey(workspaceId: string, profile: string) {
  return ["session-layout", workspaceId, profile.trim()] as const;
}

export function pendingSessionLayoutFilter(workspaceId: string, profile: string) {
  return {
    mutationKey: sessionLayoutMutationKey(workspaceId, profile),
    exact: true,
    status: "pending" as const,
  };
}

export function pendingSessionLayoutEdit(variables: unknown): PendingSessionLayoutEdit[] {
  const edit = (variables as { partitionEdit?: PendingSessionLayoutEdit } | undefined)
    ?.partitionEdit;
  return edit ? [edit] : [];
}

/** The deleted container's page remains available until the write settles. */
export function sessionListPartitionFolders(
  folders: readonly SessionFolderPayload[] | undefined,
  edits: readonly PendingSessionLayoutEdit[] | undefined
) {
  if (!folders || !edits?.length) return folders;
  const ids = new Set(folders.map(folder => folder.id));
  const removed = edits.flatMap(edit =>
    edit.removedFolder && !ids.has(edit.removedFolder.id) ? [edit.removedFolder] : []
  );
  return [...folders, ...removed];
}

/** Pending layout edits affect visible rows while cursors and totals stay server-owned. */
export function projectPartitionRows(
  pages: Iterable<readonly [string, { sessions: readonly SessionPayload[] }]>,
  edits: readonly PendingSessionLayoutEdit[] | undefined,
  manual: boolean
) {
  const { sessions, folderOf } = partitionSessionRows(pages);
  if (!edits?.length) return { sessions, folderOf };
  const byId = new Map(sessions.map(row => [row.id, row]));
  const projected = edits.reduce((rows, edit) => edit.update(rows, manual), sessions);
  for (const row of projected) {
    const previous = byId.get(row.id);
    if (previous && sessionFolderId(previous) !== sessionFolderId(row)) {
      folderOf.set(row.id, sessionFolderId(row));
    }
  }
  return { sessions: projected, folderOf };
}
