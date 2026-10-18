import type { KeyboardEvent } from "react";

import { isEditableTarget } from "../lib/editable-target";
import {
  sessionListShortcut,
  threadVisibleOrder,
  type SessionListShortcut,
  type SessionThreadModel,
} from "../lib/session-list-threads";
import type { SessionPayload } from "../types";
import type { SessionLifecycleActionHandlers } from "./use-session-lifecycle-actions";
import { sessionSelectionCounts, useSessionSelection } from "./use-session-selection";

interface SessionListBulkSelectionInput {
  scope: string;
  archived: boolean;
  sessions: readonly SessionPayload[];
  threads: readonly SessionThreadModel[];
  collapsedThreads: ReadonlySet<string>;
  sessionActions: SessionLifecycleActionHandlers;
}

type ManyHandler = ((sessions: readonly SessionPayload[]) => void) | undefined;

/** Binds an optional bulk verb to the current selection; absent verbs stay absent. */
function bindToSelection(handler: ManyHandler, selected: readonly SessionPayload[]) {
  return handler ? () => handler(selected) : undefined;
}

/**
 * The list's bulk selection: the visible row order, the per-row selection
 * contract, the selection bar's counts and verbs, and the list keyboard
 * shortcuts. `All workspaces` is read-only, so it selects nothing.
 */
export function useSessionListBulkSelection({
  scope,
  archived,
  sessions,
  threads,
  collapsedThreads,
  sessionActions,
}: SessionListBulkSelectionInput) {
  const allWorkspaces = scope === "all-workspaces";
  const selection = useSessionSelection(scope, archived, sessions);
  const catalog = allWorkspaces ? [] : sessions;
  const visibleOrder = allWorkspaces ? [] : threadVisibleOrder(threads, collapsedThreads);
  const sessionsById = new Map(catalog.map(session => [session.id, session]));
  const selectedSessions = selection.selectedIds.flatMap(id => {
    const session = sessionsById.get(id);
    return session ? [session] : [];
  });
  const selectedIdSet = new Set(selection.selectedIds);
  const actionPending = sessionActions.pendingAction !== null;
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
    if (allWorkspaces || event.defaultPrevented || isEditableTarget(event.target)) return;
    if (!event.currentTarget.contains(event.target as Node)) return;
    const shortcut = sessionListShortcut(event, selection.mode, actionPending);
    if (shortcut === null) return;
    event.preventDefault();
    event.stopPropagation();
    shortcuts[shortcut]();
  };

  return {
    /** The selection bar replaces the toolbar while rows are selected. */
    active: !allWorkspaces && selection.mode,
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
    },
    onKeyDown,
  };
}
