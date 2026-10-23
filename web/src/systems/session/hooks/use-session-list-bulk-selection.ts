import type { KeyboardEvent } from "react";

import { isEditableTarget } from "../lib/editable-target";
import { sessionListShortcut, type SessionListShortcut } from "../lib/session-list-threads";
import type { SessionPayload } from "../types";
import type { SessionLifecycleActionHandlers } from "./use-session-lifecycle-actions";
import {
  useSessionListBulkFolders,
  type SessionBulkFolderSource,
} from "./use-session-list-bulk-folders";
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
}: SessionListBulkSelectionInput) {
  const selection = useSessionSelection(selectionKey, archived, sessions);
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

  return {
    /** The selection bar replaces the toolbar while rows are selected. */
    active: selection.mode,
    rowSelection: {
      ...selection,
      selectedIds: selectedIdSet,
      toggleRange: (id: string) => selection.toggleRange(id, visibleOrder),
    },
    bar: {
      count: selection.selectedIds.length,
      ...sessionSelectionCounts(selectedSessions, visibleOrder),
      allSelected: visibleOrder.length > 0 && visibleOrder.every(id => selectedIdSet.has(id)),
      disabled: actionPending,
      onSelectAll: shortcuts["select-all"],
      onClear: selection.clear,
      onDelete: deleteSelected,
      onStop: bindToSelection(sessionActions.onStopMany, selectedSessions),
      onArchive: bindToSelection(sessionActions.onArchiveMany, selectedSessions),
      onUnarchive: bindToSelection(sessionActions.onUnarchiveMany, selectedSessions),
      folders: folderMove?.menu,
    },
    /** What dragging a selected row carries; absent without folders. */
    drag: folderMove?.drag,
    folderMove,
    onKeyDown,
  };
}
