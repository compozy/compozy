import { CircleSlash } from "lucide-react";

import { Button } from "@compozy/ui";

import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import {
  useSessionsBoardColumns,
  type SessionsBoardColumnView,
  type SessionsBoardLane,
} from "../../hooks/use-sessions-board";
import type { SessionBoardPayload } from "../../lib/session-board-contract";
import { SessionListGroupIdentity } from "../session-list/session-list-group-header";
import { SessionListGroupProjectMenu } from "../session-list/session-list-group-project-menu";
import { SessionsBoardColumn } from "./sessions-board-column";
import {
  SESSIONS_BOARD_COLUMNS_ROW,
  SESSIONS_BOARD_LANE,
  SESSIONS_BOARD_LANE_COUNT,
  SESSIONS_BOARD_LANE_HEADER,
} from "./sessions-board-frame";

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
}

/** The four derived columns side by side: the whole ungrouped board, or one lane. */
export function SessionsBoardColumnsRow({
  columns,
  doneCollapsed,
  onDoneToggle,
  layout,
  ...props
}: SessionsBoardColumnsRowProps) {
  return (
    <div className={SESSIONS_BOARD_COLUMNS_ROW[layout]}>
      {columns.map(column => (
        <SessionsBoardColumn
          {...props}
          collapsible={
            column.id === "done" ? { collapsed: doneCollapsed, onToggle: onDoneToggle } : undefined
          }
          column={column}
          key={column.id}
          layout={layout}
        />
      ))}
    </div>
  );
}

function laneName(lane: SessionsBoardLane): string {
  return lane.group.badge?.name ?? lane.group.owner?.name ?? "Group";
}

/**
 * One Group-by swimlane (GitHub Projects style): the group's identity and
 * count, then the same four derived columns read through the group's own scope
 * and profile lens, so every column count is the group's server total.
 * Cards in a project lane omit the project name its header already shows.
 */
export function SessionsBoardSwimlane({
  lane,
  ...props
}: Omit<SessionsBoardColumnsRowProps, "columns" | "layout" | "regionPrefix" | "testIdPrefix"> & {
  lane: SessionsBoardLane;
}) {
  const columns = useSessionsBoardColumns(lane.filters, true);
  const name = laneName(lane);
  const testId = `sessions-board-lane-${lane.group.key}`;
  return (
    <section aria-label={name} className={SESSIONS_BOARD_LANE} data-testid={testId}>
      <SessionListGroupProjectMenu group={lane.group}>
        <header className={SESSIONS_BOARD_LANE_HEADER} data-testid={`${testId}-header`}>
          <SessionListGroupIdentity group={lane.group} />
          <span className={SESSIONS_BOARD_LANE_COUNT} data-testid={`${testId}-count`}>
            {lane.group.total}
          </span>
        </header>
      </SessionListGroupProjectMenu>
      <SessionsBoardColumnsRow
        {...props}
        // A project lane names its project once, in its header, like a List project group.
        workspaceNameOf={lane.group.kind === "project" ? () => null : props.workspaceNameOf}
        columns={columns}
        layout="lane"
        regionPrefix={name}
        testIdPrefix={testId}
      />
    </section>
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
