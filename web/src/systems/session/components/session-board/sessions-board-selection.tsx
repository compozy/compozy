import type { ReactNode } from "react";

import { useTopbarSlot, type TopbarSlotValue } from "@compozy/ui";

import {
  SessionsBoardRowsStoreContext,
  SessionsBoardSelectionStoreContext,
  SessionsBoardSelectionVerbsContext,
} from "../../contexts/sessions-board-selection-context-value";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { useSessionsBoardSelection } from "../../hooks/use-sessions-board-selection";
import { sessionCountLabel } from "../../lib/session-folders";
import { SessionListSelectionBar } from "../session-list/session-list-selection-bar";
import { SessionListSelectionFolders } from "../session-list/session-list-selection-folders";

export interface SessionsBoardSelectionProps {
  /** The board's identity (`SessionsBoardView.selectionKey`): a new one starts an empty selection. */
  selectionKey: string;
  sessionActions: SessionLifecycleActionHandlers;
  /** The window head; while cards are selected the selection bar takes its actions' place. */
  topbar: TopbarSlotValue;
  /** The board. It renders once per window render, never per selection change. */
  children: ReactNode;
}

/**
 * The board's bulk selection host: the session list's selection, verbs and
 * shortcuts (Esc, Cmd/Ctrl+A, Cmd/Ctrl+Backspace), and its selection bar, which
 * replaces the head's actions while cards are selected, as it replaces the list
 * toolbar, so the board never shifts. This component re-renders on every
 * selection change; `children` keeps its element, so the cards re-render only
 * through their own store subscriptions. A polite status announces the count.
 */
export function SessionsBoardSelection({
  selectionKey,
  sessionActions,
  topbar,
  children,
}: SessionsBoardSelectionProps) {
  const { bulk, rows } = useSessionsBoardSelection({ selectionKey, sessionActions });
  const { count } = bulk.bar.verbs;
  useTopbarSlot({
    ...topbar,
    actions: bulk.active ? (
      // The head renders outside the board, so the bar takes the board's shortcuts too.
      <div className="contents" onKeyDown={bulk.onKeyDown}>
        <SessionListSelectionBar
          {...bulk.bar}
          className="min-h-0 px-0 py-0"
          testIdPrefix="sessions-board"
        />
      </div>
    ) : (
      topbar.actions
    ),
  });
  return (
    <>
      <SessionsBoardSelectionStoreContext value={bulk.store}>
        <SessionsBoardRowsStoreContext value={rows}>
          <SessionsBoardSelectionVerbsContext value={bulk.rowSelection.verbs}>
            <div className="contents" onKeyDown={bulk.onKeyDown}>
              {children}
            </div>
          </SessionsBoardSelectionVerbsContext>
        </SessionsBoardRowsStoreContext>
      </SessionsBoardSelectionStoreContext>
      <span
        aria-atomic="true"
        aria-live="polite"
        className="sr-only"
        data-testid="sessions-board-selection-status"
        role="status"
      >
        {count > 0 ? `${sessionCountLabel(count)} selected` : ""}
      </span>
      {bulk.folderMove ? (
        <SessionListSelectionFolders active={bulk.active} folders={bulk.folderMove} />
      ) : null}
    </>
  );
}
