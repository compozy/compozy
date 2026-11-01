import { useState } from "react";
import { toast } from "@compozy/ui";

import {
  sessionBulkMove,
  sessionBulkMoveBlockedReason,
  sessionBulkMoveIds,
  sessionCountLabel,
  sessionFolderId,
} from "../lib/session-folders";
import type { SessionListPair } from "../lib/session-list-groups";
import { sessionListPairKey } from "../lib/session-list-group-rows";
import type { SessionFolderPayload, SessionPayload } from "../types";
import { useSessionFolders, type SessionListLayout } from "./use-session-folders";
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

/**
 * Where the list's folders live: the one (workspace, profile) pair's layout of
 * a flat list, or — in a grouped list — the pair each listed session's group
 * owns, so the selection moves within the pair it sits in.
 */
export interface SessionBulkFolderSource {
  layout?: SessionListLayout;
  pairOf?: (sessionId: string) => SessionListPair | null;
}

/** A selection spanning groups names no single layout to move into. */
export const SESSION_BULK_MOVE_MIXED_PAIRS =
  "Select sessions from one project and profile to move them.";

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
  const pairs = selectionPairs(move.sessions, source.pairOf);
  const selectionPair = pairs.length === 1 ? (pairs[0] ?? null) : null;
  const owned = useSessionFolders({
    workspaceId: selectionPair?.workspaceId ?? null,
    profile: selectionPair?.profile ?? "",
    enabled: source.layout === undefined && selectionPair !== null,
  });
  const layout = source.layout ?? owned.layout;
  const mixed = source.pairOf !== undefined && pairs.length > 1;
  const blockedReason = mixed ? SESSION_BULK_MOVE_MIXED_PAIRS : sessionBulkMoveBlockedReason(move);
  if (!layout) {
    // A grouped list still says why a selection across groups cannot move.
    return mixed
      ? blockedFolderMove(
          SESSION_BULK_MOVE_MIXED_PAIRS,
          options.selectedIds,
          move.sessions.length,
          setPreview
        )
      : null;
  }
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
    pairKey: sessionListPairKey(selectionPair),
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
  return {
    menu,
    drag,
    dialog: dialog as SessionBulkFolderDialog | null,
    pending,
    previewCount: move.sessions.length,
    setPreview,
  };
}

/** The distinct pairs of the movable selection; an unowned row counts as its own pair. */
function selectionPairs(
  sessions: readonly SessionPayload[],
  pairOf: SessionBulkFolderSource["pairOf"]
): (SessionListPair | null)[] {
  if (!pairOf) return [];
  const byKey = new Map<string | null, SessionListPair | null>();
  for (const session of sessions) {
    const pair = pairOf(session.id);
    byKey.set(sessionListPairKey(pair) ?? `unowned:${session.id}`, pair);
  }
  return [...byKey.values()];
}

/** Move to folder for a selection no one layout owns: the reason, and nothing to move into. */
function blockedFolderMove(
  blockedReason: string,
  selectedIds: ReadonlySet<string>,
  count: number,
  setPreview: (element: HTMLElement | null) => void
) {
  const noop = () => undefined;
  const menu: SessionBulkFolderMenu = {
    folders: [],
    blockedReason,
    sharedFolderId: undefined,
    canUnfile: false,
    onMove: noop,
    onNewFolder: noop,
  };
  const drag: SessionSelectionDrag = {
    selectedIds,
    sessions: [],
    blockedReason,
    disabled: false,
    preview: null,
    pairKey: null,
    move: noop,
  };
  return {
    menu,
    drag,
    dialog: null as SessionBulkFolderDialog | null,
    pending: false,
    previewCount: count,
    setPreview,
  };
}

export type SessionListBulkFolders = NonNullable<ReturnType<typeof useSessionListBulkFolders>>;
