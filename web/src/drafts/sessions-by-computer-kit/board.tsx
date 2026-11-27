/**
 * The Kanban over several computers. Separate gives each computer a band; with Group by Project
 * the band holds one lane per project, the same lanes a one-computer board shows. A folded band
 * keeps its five column counts on one line. Together keeps the flat project · computer lanes.
 */
import { ChevronRight, Plus, SquareKanban } from "@compozy/ui/icons";
import { useState, type ReactNode } from "react";

import { Button, Icon, cn, type TopbarSlotValue } from "@compozy/ui";

import { ComputerPlatformIcon } from "@/systems/computers/components/computer-platform-icon";
import { ScopedComputerError } from "@/systems/computers/components/scoped-computer-status";
import type { SessionBoardCardProps } from "@/systems/session/components/session-board/session-board-card";
import { SessionBoardCardSlot } from "@/systems/session/components/session-board/session-board-card-slot";
import { SessionBoardCardView } from "@/systems/session/components/session-board/session-board-card-view";
import { SessionsBoardColumnGlyph } from "@/systems/session/components/session-board/sessions-board-column-glyph";
import { SessionsBoardColumnsRow } from "@/systems/session/components/session-board/sessions-board-columns-row";
import {
  SESSIONS_BOARD_FRAME,
  SESSIONS_BOARD_LANE,
  SESSIONS_BOARD_LANE_COUNT,
  SESSIONS_BOARD_LANE_HEADER,
  SESSIONS_BOARD_LANES,
  SESSIONS_BOARD_SCROLLER,
} from "@/systems/session/components/session-board/sessions-board-frame";
import { SessionListDisclosureMark } from "@/systems/session/components/session-list/session-list-disclosure-mark";
import { SessionViewOptionsMenu } from "@/systems/session/components/session-view-options-menu";
import type {
  SessionListComputerLayout,
  SessionListGroupBy,
} from "@/systems/session/lib/session-list-preferences";
import { boardRuntimeFixture } from "@/systems/session/mocks/sessions-board-view-fixtures";
import type { SessionPayload } from "@/systems/session/types";
import { WorkspaceBadge } from "@/systems/workspace/components/workspace-badge";

import { DraftShell } from "../always-on-agents-shared/draft-shell";
import { columnsOf, projectName } from "./fixtures";
import { byProject, type RailComputer, type RailLayout } from "./model";
import { ComputerMark, ComputerTag } from "./rail-tree";
import { ViewToggle } from "./windows";

const noop = () => undefined;

function BoardCard({ actions: _actions, ...props }: SessionBoardCardProps) {
  return <SessionBoardCardView {...props} runtime={boardRuntimeFixture(props.session)} />;
}

type ColumnFolds = Record<string, boolean>;

function Columns({
  sessions,
  folds,
  onFold,
  region,
  named,
}: {
  sessions: readonly SessionPayload[];
  folds: ColumnFolds;
  onFold: (id: string, next: boolean) => void;
  region: string;
  /** Cards name their project unless the lane already does. */
  named: boolean;
}) {
  return (
    <SessionsBoardColumnsRow
      collapsedColumns={folds}
      columns={columnsOf(sessions)}
      currentSessionId={null}
      layout="lane"
      onColumnCollapsedChange={onFold}
      onOpen={noop}
      regionPrefix={region}
      testIdPrefix={`draft-lane-${region}`}
      workspaceNameOf={session => (named ? projectName(session.workspace_id) : null)}
    />
  );
}

/** One project lane: the List's project group, as a row of columns. */
function ProjectLane({
  computer,
  workspaceId,
  sessions,
  folds,
  onFold,
  named = false,
}: {
  computer: RailComputer;
  workspaceId: string;
  sessions: readonly SessionPayload[];
  folds: ColumnFolds;
  onFold: (id: string, next: boolean) => void;
  named?: boolean;
}) {
  const [open, setOpen] = useState(true);
  return (
    <section className={SESSIONS_BOARD_LANE} data-collapsed={open ? undefined : "true"}>
      <button
        aria-expanded={open}
        className={cn(
          SESSIONS_BOARD_LANE_HEADER,
          "group/disclosure w-max rounded-sm hover:bg-surface-2"
        )}
        onClick={() => setOpen(value => !value)}
        type="button"
      >
        {/* Together: the computer's mark holds the chevron's cell at rest, as in the List. */}
        <SessionListDisclosureMark
          mark={named ? <ComputerMark computer={computer} /> : undefined}
          open={open}
        />
        <WorkspaceBadge
          className="font-medium text-fg"
          identity={{ kind: "project", name: projectName(workspaceId) }}
          variant="chrome"
        />
        {named ? <ComputerTag bare computer={computer} /> : null}
        <span className={SESSIONS_BOARD_LANE_COUNT}>{sessions.length}</span>
      </button>
      {open ? (
        <Columns
          folds={folds}
          named={false}
          onFold={onFold}
          region={`${computer.key}-${workspaceId}`}
          sessions={sessions}
        />
      ) : null}
    </section>
  );
}

/** A folded band's one line: each column's mark, name and count on that computer. */
function FoldedCounts({ sessions }: { sessions: readonly SessionPayload[] }) {
  return (
    <div className="flex flex-wrap items-center gap-x-5 gap-y-1 pl-8 text-small-body text-muted-foreground">
      {columnsOf(sessions).map(column => (
        <span className="flex items-center gap-1.5" key={column.id}>
          <SessionsBoardColumnGlyph id={column.id} />
          {column.label}
          <span className="text-subtle tabular-nums">{column.count}</span>
        </span>
      ))}
    </div>
  );
}

function BandBody({
  computer,
  groupBy,
  folds,
  onFold,
}: {
  computer: RailComputer;
  groupBy: SessionListGroupBy;
  folds: ColumnFolds;
  onFold: (id: string, next: boolean) => void;
}) {
  if (computer.read === "failed") {
    return (
      <ScopedComputerError
        className="ml-8 w-max"
        computerId={computer.key}
        computerName={computer.name}
        onRetry={noop}
      />
    );
  }
  if (computer.sessions.length === 0) {
    return (
      <p className="m-0 pl-8 text-small-body text-muted-foreground">
        No sessions on {computer.name} yet.
      </p>
    );
  }
  const dim = computer.read === "offline" ? "opacity-70" : undefined;
  if (groupBy !== "project") {
    return (
      <div className={cn("pl-6", dim)}>
        <Columns
          folds={folds}
          named
          onFold={onFold}
          region={computer.key}
          sessions={computer.sessions}
        />
      </div>
    );
  }
  return (
    <div className={cn("flex flex-col gap-1 pl-6", dim)}>
      {byProject(computer.sessions).map(([workspaceId, sessions]) => (
        <ProjectLane
          computer={computer}
          folds={folds}
          key={workspaceId}
          onFold={onFold}
          sessions={sessions}
          workspaceId={workspaceId}
        />
      ))}
    </div>
  );
}

/** One computer's band: its header, then its lanes, or its column counts when folded. */
function ComputerBand({
  computer,
  groupBy,
  initiallyFolded,
  folds,
  onFold,
}: {
  computer: RailComputer;
  groupBy: SessionListGroupBy;
  initiallyFolded: boolean;
  folds: ColumnFolds;
  onFold: (id: string, next: boolean) => void;
}) {
  const [open, setOpen] = useState(!initiallyFolded);
  const counted = computer.read === "ok" || computer.read === "offline";
  return (
    <section
      aria-label={computer.name}
      className="flex flex-col gap-2 border-t border-line-soft pt-3 first:border-t-0 first:pt-0"
    >
      <button
        aria-expanded={open}
        className="flex h-8 w-max items-center gap-2 rounded-sm px-1 hover:bg-surface-2"
        onClick={() => setOpen(value => !value)}
        type="button"
      >
        <Icon
          as={ChevronRight}
          className={cn("shrink-0 text-faint transition-transform", open && "rotate-90")}
          size="sm"
        />
        <ComputerPlatformIcon
          className="size-4 shrink-0"
          iconKind={computer.computer.iconKind}
          platform={computer.computer.platform}
          size="sm"
          tone="muted"
        />
        <span className="text-item-title font-medium text-fg">{computer.name}</span>
        <span
          className={cn("text-eyebrow", computer.read === "offline" ? "text-faint" : "text-subtle")}
        >
          {computer.detail}
        </span>
        {counted ? (
          <span className={SESSIONS_BOARD_LANE_COUNT}>{computer.sessions.length}</span>
        ) : null}
      </button>
      {open ? (
        <BandBody computer={computer} folds={folds} groupBy={groupBy} onFold={onFold} />
      ) : counted ? (
        <FoldedCounts sessions={computer.sessions} />
      ) : null}
    </section>
  );
}

function TogetherLanes({
  computers,
  folds,
  onFold,
}: {
  computers: readonly RailComputer[];
  folds: ColumnFolds;
  onFold: (id: string, next: boolean) => void;
}) {
  const groups = byProject(computers.flatMap(computer => computer.sessions)).flatMap(
    ([workspaceId]) =>
      computers.flatMap(computer => {
        const sessions = computer.sessions.filter(session => session.workspace_id === workspaceId);
        return sessions.length > 0 ? [{ computer, workspaceId, sessions }] : [];
      })
  );
  return (
    <>
      {groups.map(group => (
        <ProjectLane
          computer={group.computer}
          folds={folds}
          key={`${group.computer.key}:${group.workspaceId}`}
          named
          onFold={onFold}
          sessions={group.sessions}
          workspaceId={group.workspaceId}
        />
      ))}
    </>
  );
}

export interface BoardWindowProps {
  layout: RailLayout;
  groupBy: SessionListGroupBy;
  onGroupByChange: (next: SessionListGroupBy) => void;
  onLayoutChange: (next: SessionListComputerLayout) => void;
  computers: readonly RailComputer[];
  /** Computer keys whose band starts folded. */
  folded?: readonly string[];
  scope?: ReactNode;
}

export function BoardWindow({
  layout,
  groupBy,
  onGroupByChange,
  onLayoutChange,
  computers,
  folded = [],
  scope,
}: BoardWindowProps) {
  const [folds, setFolds] = useState<ColumnFolds>({});
  const onFold = (id: string, next: boolean) => setFolds(value => ({ ...value, [id]: next }));
  const topbar: TopbarSlotValue = {
    crumb: "Sessions",
    overflow: (
      <span className="flex items-center gap-2">
        <ViewToggle value="kanban" />
        {scope}
      </span>
    ),
    actions: (
      <>
        <SessionViewOptionsMenu
          computerLayout={
            layout === "scoped"
              ? undefined
              : {
                  value: layout === "separate" ? "separate" : "together",
                  onValueChange: onLayoutChange,
                }
          }
          groupBy={groupBy}
          groupByOptions={["none", "project", "profile"]}
          onGroupByChange={onGroupByChange}
          onShowEmptyProjectsChange={noop}
          onSortChange={noop}
          showEmptyProjects={false}
          sort="attention"
          sortOptions={["created", "name", "last_activity", "attention"]}
          testId="draft-board"
        />
        <Button size="sm" type="button" variant="secondary">
          <Plus aria-hidden="true" data-icon="inline-start" />
          New session
        </Button>
      </>
    ),
  };
  const foldedKeys = new Set(folded);
  let body: ReactNode;
  if (layout === "separate") {
    body = (
      <div className={cn(SESSIONS_BOARD_LANES, "gap-4")}>
        {computers.map(computer => (
          <ComputerBand
            computer={computer}
            folds={folds}
            groupBy={groupBy}
            initiallyFolded={foldedKeys.has(computer.key)}
            key={computer.key}
            onFold={onFold}
          />
        ))}
      </div>
    );
  } else if (layout === "together" && groupBy === "project") {
    body = (
      <div className={SESSIONS_BOARD_LANES}>
        <TogetherLanes computers={computers} folds={folds} onFold={onFold} />
      </div>
    );
  } else {
    const sessions = computers.flatMap(computer => computer.sessions);
    body =
      groupBy === "project" ? (
        <div className={SESSIONS_BOARD_LANES}>
          {byProject(sessions).map(([workspaceId, rows]) => (
            <ProjectLane
              computer={computers[0]!}
              folds={folds}
              key={workspaceId}
              onFold={onFold}
              sessions={rows}
              workspaceId={workspaceId}
            />
          ))}
        </div>
      ) : (
        <Columns folds={folds} named onFold={onFold} region="board" sessions={sessions} />
      );
  }
  return (
    <DraftShell
      app="sessions"
      glyph={<SquareKanban />}
      sessionBadge={4}
      title="Sessions"
      topbar={topbar}
    >
      <SessionBoardCardSlot value={BoardCard}>
        <div className={SESSIONS_BOARD_FRAME}>
          <div className={SESSIONS_BOARD_SCROLLER}>{body}</div>
        </div>
      </SessionBoardCardSlot>
    </DraftShell>
  );
}
