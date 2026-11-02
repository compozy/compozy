import { useSelector, useStore } from "@xstate/store-react";

import type { SessionListGroupSelection } from "../contexts/session-list-group-selection-context-value";
import { sessionListGroupedRows, sessionListGroupRowsLogic } from "../lib/session-list-group-rows";
import { threadVisibleOrder, type SessionThreadModel } from "../lib/session-list-threads";
import type { SessionListShelf } from "../lib/session-list-shelf";
import type { SessionPayload } from "../types";
import type { SessionListLayout } from "./use-session-folders";
import type { SessionLifecycleActionHandlers } from "./use-session-lifecycle-actions";
import { useSessionListBulkSelection } from "./use-session-list-bulk-selection";
import type { SessionFoldSet } from "../lib/session-sidebar-scope";

/**
 * The list's one bulk selection. A flat list selects its own catalog in its
 * visible order and moves within its one layout; a grouped list joins the rows
 * each expanded group publishes, in on-screen group order, and moves within
 * the pair the selection sits in.
 */
export function useSessionListSelection(input: {
  selectionKey: string;
  shelf: SessionListShelf;
  grouped: boolean;
  /** The flat list's sessions, visible threads, and folder layout. */
  sessions: readonly SessionPayload[];
  visibleThreads: readonly SessionThreadModel[];
  collapsedThreads: SessionFoldSet;
  layout?: SessionListLayout;
  sessionActions: SessionLifecycleActionHandlers;
}) {
  const rows = useStore(sessionListGroupRowsLogic);
  const grouped = sessionListGroupedRows(useSelector(rows, snapshot => snapshot.context.groups));
  const bulk = useSessionListBulkSelection({
    selectionKey: input.selectionKey,
    shelf: input.shelf,
    sessions: input.grouped ? grouped.sessions : input.sessions,
    visibleOrder: input.grouped
      ? grouped.visibleOrder
      : threadVisibleOrder(input.visibleThreads, input.collapsedThreads),
    sessionActions: input.sessionActions,
    folders: input.grouped ? { pairOf: grouped.pairOf } : { layout: input.layout },
  });
  const groupSelection: SessionListGroupSelection = {
    rows,
    selection: bulk.rowSelection,
    drag: bulk.drag,
  };
  return { bulk, groupSelection };
}
