import { ChevronRight } from "@compozy/ui/icons";

import { Icon, StatusDot, cn } from "@compozy/ui";

import { openContextMenuFromKeyboard } from "@/systems/os";

import { useSessionsBoardColumns, type SessionsBoardLane } from "../../hooks/use-sessions-board";
import type { SessionBoardPayload } from "../../lib/session-board-contract";
import {
  sessionListGroupTarget,
  sessionListGroupEmptyMessage,
  sessionListGroupSignalLabel,
} from "../../lib/session-list-groups";
import {
  SessionListGroupIdentity,
  SessionListGroupSignal,
} from "../session-list/session-list-group-header";
import { SessionListGroupProjectMenu } from "../session-list/session-list-group-project-menu";
import { AlwaysOnProjectRow } from "../always-on/always-on-signals";
import {
  SessionsBoardColumnsRow,
  type SessionsBoardColumnsRowProps,
} from "./sessions-board-columns-row";
import {
  SESSIONS_BOARD_LANE,
  SESSIONS_BOARD_LANE_COUNT,
  SESSIONS_BOARD_LANE_HEADER,
  SESSIONS_BOARD_LANE_SELECT_SLOT,
} from "./sessions-board-frame";
import { SessionsBoardSelectAll } from "./sessions-board-select-all";

/** The board header has room for a full crew. */
const BOARD_ROW_LIMIT = 12;

function laneName(lane: SessionsBoardLane): string {
  return lane.group.badge?.name ?? lane.group.owner?.name ?? "Group";
}

/**
 * One Group-by swimlane (GitHub Projects style): the group's identity and
 * count, then the same derived columns read through the group's own scope
 * and profile lens, so every column count is the group's server total.
 * Cards in a project lane omit the project name its header already shows.
 * A folded or empty lane is its header alone and reads no columns.
 */
export function SessionsBoardSwimlane({
  lane,
  ...props
}: Omit<SessionsBoardColumnsRowProps, "columns" | "layout" | "regionPrefix" | "testIdPrefix"> & {
  lane: SessionsBoardLane;
}) {
  const empty = lane.group.total === 0;
  const open = !lane.collapsed && !empty;
  const columns = useSessionsBoardColumns(lane.filters, open);
  const name = laneName(lane);
  const testId = `sessions-board-lane-${lane.group.key}`;
  const alwaysOnScope =
    lane.group.kind === "project" ? sessionListGroupTarget(lane.group).workspaceId : null;
  // The lane's select-all covers the cards its columns show; a folded column shows none.
  const shownIds = open
    ? columns.flatMap(column =>
        props.collapsedColumns[column.id] === true ? [] : column.sessions.map(session => session.id)
      )
    : [];
  return (
    <section
      aria-label={name}
      className={SESSIONS_BOARD_LANE}
      data-collapsed={lane.collapsed ? "true" : undefined}
      data-testid={testId}
    >
      <div className="group/board-select flex items-center gap-1">
        <SessionsBoardSelectAll ids={shownIds} label={name} testId={`${testId}-select`}>
          <span className={SESSIONS_BOARD_LANE_SELECT_SLOT} />
        </SessionsBoardSelectAll>
        <SessionsBoardLaneToggle lane={lane} name={name} testId={testId} />
        {alwaysOnScope ? (
          <AlwaysOnProjectRow
            className="flex-1 pl-2"
            currentSessionId={props.currentSessionId}
            limit={BOARD_ROW_LIMIT}
            name={name}
            onOpenSession={session => props.onOpen(session as SessionBoardPayload)}
            surface="ring-canvas"
            workspaceId={alwaysOnScope}
          />
        ) : null}
      </div>
      {open ? (
        <SessionsBoardColumnsRow
          {...props}
          // A project lane names its project once, in its header, like a List project group.
          workspaceNameOf={lane.group.kind === "project" ? () => null : props.workspaceNameOf}
          columns={columns}
          layout="lane"
          regionPrefix={name}
          testIdPrefix={testId}
        />
      ) : null}
      {empty && !lane.collapsed ? (
        <p className="px-1 text-small-body text-muted-foreground" data-testid={`${testId}-empty`}>
          {sessionListGroupEmptyMessage(lane.group, "active")}
        </p>
      ) : null}
    </section>
  );
}

function SessionsBoardLaneToggle({
  lane,
  name,
  testId,
}: {
  lane: SessionsBoardLane;
  name: string;
  testId: string;
}) {
  const waiting = lane.group.instancesNeedingAttention;
  const projectLane = lane.group.kind === "project" && lane.group.scope.kind === "project";
  const signal = lane.collapsed ? sessionListGroupSignalLabel(lane.group) : null;
  return (
    <SessionListGroupProjectMenu group={lane.group}>
      <header className="min-w-0">
        <button
          aria-expanded={!lane.collapsed}
          className={cn(
            SESSIONS_BOARD_LANE_HEADER,
            "rounded-sm transition-colors outline-none hover:bg-surface-2 focus-visible:shadow-focus-ring"
          )}
          data-testid={`${testId}-header`}
          onClick={lane.onToggle}
          type="button"
          {...(projectLane
            ? {
                "aria-label": signal ? `${name} project, ${signal}` : `${name} project`,
                onKeyDown: openContextMenuFromKeyboard,
              }
            : {})}
        >
          <Icon
            as={ChevronRight}
            className={cn(
              "shrink-0 text-faint transition-transform",
              !lane.collapsed && "rotate-90"
            )}
            size="sm"
          />
          <SessionListGroupIdentity group={lane.group} />
          {lane.collapsed ? <SessionListGroupSignal group={lane.group} /> : null}
          <span className={SESSIONS_BOARD_LANE_COUNT} data-testid={`${testId}-count`}>
            {lane.group.total}
          </span>
          {waiting > 0 ? (
            <span
              className="flex items-center gap-1.5 text-small-body text-muted-foreground"
              data-testid={`${testId}-agents-waiting`}
            >
              <StatusDot size="sm" tone="attn" />
              {waiting === 1 ? "1 agent needs you" : `${waiting} agents need you`}
            </span>
          ) : null}
        </button>
      </header>
    </SessionListGroupProjectMenu>
  );
}
