import { useState } from "react";
import { toast } from "sonner";

import {
  sessionBulkMove,
  sessionBulkMoveBlockedReason,
  sessionBulkMoveIds,
  sessionCountLabel,
  sessionFolderId,
} from "../lib/session-folders";
import type { SessionFolderPayload, SessionPayload } from "../types";
import type { SessionListLayout } from "./use-session-folders";
import type { SessionSelectionDrag } from "./use-session-list-dnd";

/** The selection bar's Move to folder submenu. */
export interface SessionBulkFolderMenu {
  folders: readonly SessionFolderPayload[];
  /** Why the selection cannot move, shown inside the submenu instead of targets. */
  blockedReason: string | null;
  /** The folder every movable session already sits in; moving there changes nothing. */
  sharedFolderId: string | null | undefined;
  /** At least one movable session sits in a folder, so No folder applies. */
  canUnfile: boolean;
  onMove: (folderId: string | null) => void;
  onNewFolder: () => void;
}

/** The New folder dialog opened from the selection: it creates the folder, then moves into it. */
export interface SessionBulkFolderDialog {
  open: boolean;
  count: number;
  layout: SessionListLayout;
  onClose: () => void;
  onCreate: (name: string) => Promise<void>;
}

/** Where the list's folders live: the one (workspace, profile) pair's layout, when the list is one. */
export interface SessionBulkFolderSource {
  layout?: SessionListLayout;
}

/**
 * Moves the list selection into a folder from the selection bar menu or by
 * dragging a selected row. Every move is one atomic write that changes only
 * the sessions not already in the target; success announces the result and
 * ends the selection, while a refused write rolls back and keeps it.
 *
 * A folder belongs to one profile's layout, so a selection spanning profiles
 * cannot move. Subagent rows follow their parent and never move on their own.
 */
export function useSessionListBulkFolders(options: {
  source: SessionBulkFolderSource;
  sessions: readonly SessionPayload[];
  selectedIds: ReadonlySet<string>;
  visibleOrder: readonly string[];
  /** Another bulk operation is running. */
  busy: boolean;
  onMoved: () => void;
}) {
  const { source } = options;
  const move = sessionBulkMove(options.sessions, options.selectedIds, options.visibleOrder);
  const [pending, setPending] = useState(false);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [preview, setPreview] = useState<HTMLElement | null>(null);
  const layout = source.layout;
  if (!layout) return null;
  const blockedReason = sessionBulkMoveBlockedReason(move);
  const folderIds = move.sessions.map(sessionFolderId);
  /** Writes the move of every selected root not already in `folderId`; resolves once settled. */
  const run = (target: SessionListLayout, folderId: string | null, name?: string) => {
    const ids = sessionBulkMoveIds(move.sessions, folderId);
    if (ids.length === 0) return Promise.resolve();
    setPending(true);
    return target.moveSessions(ids, folderId).then(
      () => {
        setPending(false);
        const destination = folderId === null ? "out of folders" : `to “${name ?? "folder"}”`;
        toast.success(`Moved ${sessionCountLabel(ids.length)} ${destination}`);
        options.onMoved();
      },
      // The layout write already rolled the list back and reported why; the
      // selection stays so the operator can retry.
      () => setPending(false)
    );
  };
  /** The menu and drop entry point: moves the selection unless it is blocked or busy. */
  const moveTo = (folderId: string | null) => {
    if (blockedReason !== null || pending || options.busy) return;
    const name = layout.folders.find(folder => folder.id === folderId)?.name;
    void run(layout, folderId, name);
  };

  const menu: SessionBulkFolderMenu = {
    folders: layout.folders,
    blockedReason,
    sharedFolderId: folderIds.every(id => id === folderIds[0]) ? folderIds[0] : undefined,
    canUnfile: folderIds.some(id => id !== null),
    onMove: moveTo,
    onNewFolder: () => setDialogOpen(true),
  };
  const drag: SessionSelectionDrag = {
    selectedIds: options.selectedIds,
    sessions: move.sessions.map((session, index) => ({
      id: session.id,
      folderId: folderIds[index] ?? null,
    })),
    blockedReason,
    disabled: pending || options.busy,
    preview,
    move: moveTo,
  };
  const dialog: SessionBulkFolderDialog = {
    open: dialogOpen,
    count: move.sessions.length,
    layout,
    onClose: () => setDialogOpen(false),
    // A refused create keeps the dialog open with the daemon's reason. Once
    // the folder exists the dialog closes; a refused move then reads like
    // any other and keeps the selection for a retry into the new folder.
    onCreate: async name => {
      const folder = await layout.createFolder(name);
      await run(layout, folder.id, folder.name);
    },
  };
  return { menu, drag, dialog, pending, previewCount: move.sessions.length, setPreview };
}

export type SessionListBulkFolders = NonNullable<ReturnType<typeof useSessionListBulkFolders>>;
