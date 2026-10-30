import { useLayoutEffect, useRef, type KeyboardEvent } from "react";

import { isEditableTarget } from "../lib/editable-target";
import { sessionListShortcut, type SessionListShortcut } from "../lib/session-list-threads";
import type { SessionPayload } from "../types";
import type { SessionLifecycleActionHandlers } from "./use-session-lifecycle-actions";
import {
  useSessionListBulkFolders,
  type SessionBulkFolderMenu,
  type SessionBulkFolderSource,
} from "./use-session-list-bulk-folders";
import { copySessionText } from "../lib/session-copy";
import { sessionSelectionCounts, useSessionSelection } from "./use-session-selection";

interface SessionListBulkSelectionInput {
  /** The list's identity: a new breadth, lens, or grouping is a new selection. */
  selectionKey: string;
  archived: boolean;
  /** Every listed session; a grouped list joins what each expanded group reads. */
  sessions: readonly SessionPayload[];
  /** The ids of the rows on screen, in order; ranges and select-all follow it. */
  visibleOrder: readonly string[];
  sessionActions: SessionLifecycleActionHandlers;
  /** The list's folder layouts; absent while the list has no folders to move into. */
  folders: SessionBulkFolderSource;
  /**
   * Keeps selected sessions missing from `sessions` until their absence is confirmed. A
   * session moving between reads (a board card changing column) is briefly in neither.
   */
  holdMissing?: (missingIds: readonly string[]) => boolean;
}

/** What the selection can do, shared by the selection bar's menu and a selected row's context menu. */
export interface SessionSelectionVerbs {
  count: number;
  stoppable: number;
  archivable: number;
  /** Archivable sessions that archiving stops first. */
  archivableLive: number;
  unarchivable: number;
  disabled: boolean;
  onDelete?: () => void;
  onStop?: () => void;
  onArchive?: () => void;
  onUnarchive?: () => void;
  onCopyIds: () => void;
  /** Move to folder; absent while the list has no folders to move into. */
  folders?: SessionBulkFolderMenu;
}

type ManyHandler = ((sessions: readonly SessionPayload[]) => void) | undefined;

/** Binds an optional bulk verb to the current selection; absent verbs stay absent. */
function bindToSelection(handler: ManyHandler, selected: readonly SessionPayload[]) {
  return handler ? () => handler(selected) : undefined;
}

/**
 * The list's bulk selection: the visible row order, the per-row selection
 * contract, the selection bar's counts and verbs, folder moves by menu and
 * drag, and the list keyboard shortcuts. A grouped list keeps one selection
 * across its groups: lifecycle verbs route each session to its owner, and a
 * folder move needs the selection inside one group's pair.
 */
export function useSessionListBulkSelection({
  selectionKey,
  archived,
  sessions,
  visibleOrder,
  sessionActions,
  folders,
  holdMissing,
}: SessionListBulkSelectionInput) {
  const selection = useSessionSelection(selectionKey, archived, sessions, holdMissing);
  // Ranges read the order on screen when the operator clicks. Holding it outside
  // the row contract keeps a fold, which reorders nothing a row shows, from
  // handing every row a new selection object.
  const visibleOrderRef = useRef(visibleOrder);
  useLayoutEffect(() => {
    visibleOrderRef.current = visibleOrder;
  }, [visibleOrder]);
  const sessionsById = new Map(sessions.map(session => [session.id, session]));
  const selectedSessions = selection.selectedIds.flatMap(id => {
    const session = sessionsById.get(id);
    return session ? [session] : [];
  });
  const selectedIdSet = new Set(selection.selectedIds);
  const lifecyclePending = sessionActions.pendingAction !== null;
  const folderMove = useSessionListBulkFolders({
    source: folders,
    sessions,
    selectedIds: selectedIdSet,
    visibleOrder,
    busy: lifecyclePending,
    onMoved: selection.clear,
  });
  const actionPending = lifecyclePending || (folderMove?.pending ?? false);
  const { onDeleteMany } = sessionActions;
  const deleteSelected = onDeleteMany
    ? () =>
        onDeleteMany(selectedSessions, remainingIds => {
          selection.prune(remainingIds);
          if (remainingIds.length === 0) selection.clear();
        })
    : undefined;
  const shortcuts: Record<SessionListShortcut, () => void> = {
    clear: selection.clear,
    "select-all": () => selection.selectAll(visibleOrder),
    delete: () => deleteSelected?.(),
  };

  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.defaultPrevented || isEditableTarget(event.target)) return;
    if (!event.currentTarget.contains(event.target as Node)) return;
    const shortcut = sessionListShortcut(event, selection.mode, actionPending);
    if (shortcut === null) return;
    event.preventDefault();
    event.stopPropagation();
    shortcuts[shortcut]();
  };

  const counts = sessionSelectionCounts(selectedSessions, visibleOrder);
  const verbs: SessionSelectionVerbs = {
    // Held sessions have no payload to act on, so the count names what the verbs receive.
    count: selectedSessions.length,
    stoppable: counts.stoppable,
    archivable: counts.archivable,
    archivableLive: counts.archivableLive,
    unarchivable: counts.unarchivable,
    disabled: actionPending,
    onDelete: deleteSelected,
    onStop: bindToSelection(sessionActions.onStopMany, selectedSessions),
    onArchive: bindToSelection(sessionActions.onArchiveMany, selectedSessions),
    onUnarchive: bindToSelection(sessionActions.onUnarchiveMany, selectedSessions),
    onCopyIds: () =>
      void copySessionText(selectedSessions.map(session => session.id).join("\n"), "Session IDs"),
    folders: folderMove?.menu,
  };

  return {
    /** The selection bar replaces the toolbar while rows are selected. */
    active: selection.mode,
    /** The selection store, for hosts whose items subscribe to their own membership. */
    store: selection.store,
    rowSelection: {
      ...selection,
      selectedIds: selectedIdSet,
      toggleRange: (id: string) => selection.toggleRange(id, visibleOrderRef.current),
      verbs,
    },
    bar: {
      verbs,
      hiddenByFilter: counts.hiddenByFilter,
      allSelected: visibleOrder.length > 0 && visibleOrder.every(id => selectedIdSet.has(id)),
      onSelectAll: shortcuts["select-all"],
      onClear: selection.clear,
    },
    /** What dragging a selected row carries; absent without folders. */
    drag: folderMove?.drag,
    folderMove,
    onKeyDown,
  };
}
