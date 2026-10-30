import { CircleSlash } from "lucide-react";

import { Button } from "@compozy/ui";

import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import type { SessionsBoardColumnView } from "../../hooks/use-sessions-board";
import type { SessionBoardPayload } from "../../lib/session-board-contract";
import { sessionListGroupRank } from "../../lib/session-list-group-rows";
import { SessionsBoardColumn } from "./sessions-board-column";
import { SESSIONS_BOARD_COLUMNS_ROW } from "./sessions-board-frame";

export interface SessionsBoardColumnsRowProps {
  columns: SessionsBoardColumnView[];
  currentSessionId: string | null;
  workspaceNameOf: (session: SessionBoardPayload) => string | null;
  onOpen: (session: SessionBoardPayload) => void;
  actions?: SessionLifecycleActionHandlers;
  /** Done folds in every row at once, so the lanes' columns stay aligned. */
  doneCollapsed: boolean;
  onDoneToggle: () => void;
  layout: "fill" | "lane";
  regionPrefix?: string;
  testIdPrefix?: string;
  /** The row's place in the board selection: the whole board or one lane. */
  selectionRow?: { key: string; rank: number };
}

/** The four derived columns side by side: the whole ungrouped board, or one lane. */
export function SessionsBoardColumnsRow({
  columns,
  doneCollapsed,
  onDoneToggle,
  layout,
  selectionRow,
  ...props
}: SessionsBoardColumnsRowProps) {
  return (
    <div className={SESSIONS_BOARD_COLUMNS_ROW[layout]}>
      {columns.map((column, index) => (
        <SessionsBoardColumn
          {...props}
          collapsible={
            column.id === "done" ? { collapsed: doneCollapsed, onToggle: onDoneToggle } : undefined
          }
          column={column}
          key={column.id}
          layout={layout}
          selectionRows={
            selectionRow
              ? {
                  key: `${selectionRow.key}:${column.id}`,
                  rank: sessionListGroupRank([selectionRow.rank, index]),
                }
              : undefined
          }
        />
      ))}
    </div>
  );
}

/** The lanes' facets read failed: no lane can be listed until a retry. */
export function SessionsBoardLanesFailure({ onRetry }: { onRetry: () => void }) {
  return (
    <div
      className="flex flex-col items-center justify-center gap-2.5 px-3 py-8 text-center text-small-body text-muted"
      data-testid="sessions-board-lanes-error"
      role="alert"
    >
      <CircleSlash aria-hidden="true" className="size-4 text-subtle" />
      <span>Couldn't load sessions.</span>
      <Button onClick={onRetry} size="sm" type="button" variant="secondary">
        Retry
      </Button>
    </div>
  );
}
