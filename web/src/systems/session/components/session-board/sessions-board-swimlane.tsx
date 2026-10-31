import { cn } from "@compozy/ui";

import { openContextMenuFromKeyboard } from "@/systems/os";

import { useSessionsBoardColumns, type SessionsBoardLane } from "../../hooks/use-sessions-board";
import { SessionListGroupIdentity } from "../session-list/session-list-group-header";
import { SessionListGroupProjectMenu } from "../session-list/session-list-group-project-menu";
import {
  SessionsBoardColumnsRow,
  type SessionsBoardColumnsRowProps,
} from "./sessions-board-columns-row";
import {
  SESSIONS_BOARD_LANE,
  SESSIONS_BOARD_LANE_COUNT,
  SESSIONS_BOARD_LANE_HEADER,
} from "./sessions-board-frame";
import { SessionsBoardSelectAll } from "./sessions-board-select-all";

function laneName(lane: SessionsBoardLane): string {
  return lane.group.badge?.name ?? lane.group.owner?.name ?? "Group";
}

/**
 * One Group-by swimlane (GitHub Projects style): the group's identity and
 * count, then the same derived columns read through the group's own scope
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
  // A project lane's header is a project surface: it takes focus so Shift+F10 reaches its menu.
  const projectLane = lane.group.kind === "project" && lane.group.scope.kind === "project";
  // The lane's select-all covers the cards its columns show; a folded column shows none.
  const shownIds = columns.flatMap(column =>
    props.collapsedColumns[column.id] === true ? [] : column.sessions.map(session => session.id)
  );
  return (
    <section aria-label={name} className={SESSIONS_BOARD_LANE} data-testid={testId}>
      <div className="group/board-select flex items-center gap-1">
        <SessionsBoardSelectAll ids={shownIds} label={name} testId={`${testId}-select`} />
        <SessionListGroupProjectMenu group={lane.group}>
          <header
            className={cn(
              SESSIONS_BOARD_LANE_HEADER,
              projectLane && "rounded-sm outline-none focus-visible:shadow-focus-ring"
            )}
            data-testid={`${testId}-header`}
            {...(projectLane
              ? {
                  tabIndex: 0,
                  "aria-label": `${name} project`,
                  onKeyDown: openContextMenuFromKeyboard,
                }
              : {})}
          >
            <SessionListGroupIdentity group={lane.group} />
            <span className={SESSIONS_BOARD_LANE_COUNT} data-testid={`${testId}-count`}>
              {lane.group.total}
            </span>
          </header>
        </SessionListGroupProjectMenu>
      </div>
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
