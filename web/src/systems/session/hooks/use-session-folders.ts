import {
  useMutation,
  useQuery,
  useQueryClient,
  type InfiniteData,
  type QueryKey,
} from "@tanstack/react-query";
import { toast } from "@compozy/ui";

import {
  createSessionFolder,
  deleteSessionFolder,
  moveSessionToFolder,
  renameSessionFolder,
  reorderSessionFolders,
  reorderSessions,
  type SessionLayoutScope,
} from "../adapters/session-folders-api";
import { sessionKeys } from "../lib/query-keys";
import { sessionFoldersOptions } from "../lib/session-catalog-options";
import {
  applyFolderRemoval,
  applySessionMove,
  applySessionOrder,
  applySessionsMovedFirst,
  shiftId,
} from "../lib/session-folders";
import type { SessionCatalogPageResponse, SessionFolderPayload, SessionPayload } from "../types";

/** Folder layout of the workspace list plus the operator's layout verbs. */
export interface SessionListLayout {
  folders: readonly SessionFolderPayload[];
  createFolder: (name: string) => Promise<SessionFolderPayload>;
  renameFolder: (folderId: string, name: string) => Promise<void>;
  deleteFolder: (folderId: string) => Promise<void>;
  /** Keyboard alternative to dragging a folder header. */
  moveFolder: (folderId: string, delta: -1 | 1) => void;
  reorderFolders: (folderIds: readonly string[]) => void;
  /** Moves a session to the end of a folder, or back to the root when `folderId` is null. */
  moveSession: (sessionId: string, folderId: string | null) => void;
  /**
   * Moves several sessions into a folder, or the root when `folderId` is null, in
   * one atomic write. They land first in the container, in the listed order.
   * Rejects after the daemon refuses; the failure is already reported.
   */
  moveSessions: (sessionIds: readonly string[], folderId: string | null) => Promise<void>;
  /** Pins a container's full visible order; `folderId` null is the workspace root. */
  reorderSessions: (folderId: string | null, sessionIds: readonly string[]) => void;
}

/** The folder layout once loaded, or the failed first read and its retry. */
export interface SessionFoldersState {
  layout?: SessionListLayout;
  /** Set when the folder list could not be read; the list stays flat until a retry loads it. */
  loadError?: { retry: () => void };
}

type CatalogEnvelope = InfiniteData<SessionCatalogPageResponse>;
type Snapshot = { key: QueryKey; data: unknown }[];

/**
 * Session folders for one workspace and acting profile. Layout writes patch the
 * served catalog page and folder list optimistically, restore both envelopes
 * when the daemon refuses, and reread the workspace list family either way.
 */
export function useSessionFolders(options: {
  workspaceId: string | null;
  profile: string;
  enabled: boolean;
}): SessionFoldersState {
  const queryClient = useQueryClient();
  const workspaceId = options.workspaceId ?? "";
  const scope: SessionLayoutScope = { workspaceId, profile: options.profile || undefined };
  const enabled = options.enabled && workspaceId !== "";
  const foldersQuery = useQuery({ ...sessionFoldersOptions(scope), enabled });
  const foldersKey = sessionKeys.folders(workspaceId, options.profile);
  const catalogPrefix = [...sessionKeys.workspaceLists(workspaceId), "page"] as const;

  const snapshot = (): Snapshot => [
    ...queryClient
      .getQueriesData({ queryKey: catalogPrefix })
      .map(([key, data]) => ({ key, data })),
    { key: foldersKey, data: queryClient.getQueryData(foldersKey) },
  ];
  /** Patches every served catalog page; `manual` says whether that page is in Manual sort. */
  const patchSessions = (
    update: (sessions: readonly SessionPayload[], manual: boolean) => SessionPayload[]
  ) => {
    for (const [key, envelope] of queryClient.getQueriesData<CatalogEnvelope>({
      queryKey: catalogPrefix,
    })) {
      if (!envelope) continue;
      const manual = (key.at(-1) as { sort?: string } | undefined)?.sort === "manual";
      queryClient.setQueryData<CatalogEnvelope>(key, {
        ...envelope,
        pages: envelope.pages.map(page => ({ ...page, sessions: update(page.sessions, manual) })),
      });
    }
  };
  const patchFolders = (update: (folders: SessionFolderPayload[]) => SessionFolderPayload[]) => {
    queryClient.setQueryData<SessionFolderPayload[]>(foldersKey, folders =>
      folders ? update(folders) : folders
    );
  };

  const mutation = useMutation({
    // One scope per workspace and profile runs layout writes one at a time, so
    // rapid reorders reach the daemon in the order the operator made them.
    scope: { id: `session-layout:${workspaceId}:${options.profile.trim()}` },
    mutationFn: (run: LayoutWrite) => run.request(),
    onMutate: async (run: LayoutWrite) => {
      await queryClient.cancelQueries({ queryKey: sessionKeys.workspaceLists(workspaceId) });
      const previous = snapshot();
      run.optimistic?.();
      return { previous };
    },
    onError: (error, run, context) => {
      for (const entry of context?.previous ?? []) queryClient.setQueryData(entry.key, entry.data);
      toast.error(run.failure, { description: error instanceof Error ? error.message : undefined });
    },
    onSettled: () =>
      queryClient.invalidateQueries({ queryKey: sessionKeys.workspaceLists(workspaceId) }),
  });
  const write = (run: LayoutWrite) => mutation.mutateAsync(run).then(() => undefined);
  const fireAndForget = (run: LayoutWrite) => void write(run).catch(() => undefined);

  // Without the folder list, foldered rows would render as root members and a
  // root reorder would unfile them, so the list stays flat until it loads.
  if (!enabled) return {};
  if (foldersQuery.data === undefined) {
    return foldersQuery.isError ? { loadError: { retry: () => void foldersQuery.refetch() } } : {};
  }
  const folders = foldersQuery.data;

  const folderOrderWrite = (folderIds: readonly string[]): LayoutWrite => ({
    failure: "Couldn't reorder the folders.",
    request: () => reorderSessionFolders(scope, folderIds),
    optimistic: () =>
      patchFolders(current => {
        const byId = new Map(current.map(folder => [folder.id, folder]));
        return folderIds.flatMap((id, position) => {
          const folder = byId.get(id);
          return folder ? [{ ...folder, position }] : [];
        });
      }),
  });

  const layout: SessionListLayout = {
    folders,
    createFolder: async name => {
      const folder = await createSessionFolder(scope, name);
      patchFolders(current => [folder, ...current]);
      await queryClient.invalidateQueries({ queryKey: foldersKey });
      return folder;
    },
    renameFolder: (folderId, name) =>
      write({
        failure: "Couldn't rename the folder.",
        request: () => renameSessionFolder(scope, folderId, name),
        optimistic: () =>
          patchFolders(current =>
            current.map(folder => (folder.id === folderId ? { ...folder, name } : folder))
          ),
      }),
    deleteFolder: folderId =>
      write({
        failure: "Couldn't delete the folder.",
        request: () => deleteSessionFolder(scope, folderId),
        optimistic: () => {
          patchFolders(current => current.filter(folder => folder.id !== folderId));
          patchSessions(sessions => applyFolderRemoval(sessions, folderId));
        },
      }),
    moveFolder: (folderId, delta) => {
      const next = shiftId(
        folders.map(folder => folder.id),
        folderId,
        delta
      );
      if (next) fireAndForget(folderOrderWrite(next));
    },
    reorderFolders: folderIds => fireAndForget(folderOrderWrite(folderIds)),
    moveSession: (sessionId, folderId) =>
      fireAndForget({
        failure: "Couldn't move the session.",
        request: () => moveSessionToFolder(scope, sessionId, folderId),
        optimistic: () =>
          patchSessions(sessions => applySessionMove(sessions, sessionId, folderId)),
      }),
    moveSessions: (sessionIds, folderId) =>
      write({
        failure: "Couldn't move the sessions.",
        request: () => reorderSessions(scope, folderId, sessionIds),
        optimistic: () =>
          patchSessions((sessions, manual) =>
            manual
              ? applySessionsMovedFirst(sessions, folderId, sessionIds)
              : applySessionOrder(sessions, folderId, sessionIds)
          ),
      }),
    reorderSessions: (folderId, sessionIds) =>
      fireAndForget({
        failure: "Couldn't reorder the sessions.",
        request: () => reorderSessions(scope, folderId, sessionIds),
        optimistic: () =>
          patchSessions(sessions => applySessionOrder(sessions, folderId, sessionIds)),
      }),
  };
  return { layout };
}

interface LayoutWrite {
  failure: string;
  request: () => Promise<unknown>;
  optimistic?: () => void;
}
