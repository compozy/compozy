/** S15: the real board columns and cards inside project lanes, with the row in each header. */
import { ListTree, Plus, SquareKanban } from "lucide-react";
import { useState, type ReactNode } from "react";

import { Button, Eyebrow, cn, type TopbarSlotValue } from "@compozy/ui";

import { SessionBoardCardSlot } from "@/systems/session/components/session-board/session-board-card-slot";
import type { SessionBoardCardProps } from "@/systems/session/components/session-board/session-board-card";
import { SessionBoardCardView } from "@/systems/session/components/session-board/session-board-card-view";
import { SessionViewToggle } from "@/systems/session/components/session-board/session-view-toggle";
import { SessionsBoardColumnsRow } from "@/systems/session/components/session-board/sessions-board-columns-row";
import {
  SESSIONS_BOARD_FRAME,
  SESSIONS_BOARD_LANE,
  SESSIONS_BOARD_LANE_COUNT,
  SESSIONS_BOARD_LANE_HEADER,
  SESSIONS_BOARD_LANES,
  SESSIONS_BOARD_SCROLLER,
} from "@/systems/session/components/session-board/sessions-board-frame";
import type { SessionsBoardColumnView } from "@/systems/session/hooks/use-sessions-board";
import { boardRuntimeFixture } from "@/systems/session/mocks/sessions-board-view-fixtures";
import { WorkspaceBadge } from "@/systems/workspace/components/workspace-badge";

import { AgentAvatar } from "../stays-on-agents-shared/agent-avatar";
import { DraftShell } from "../stays-on-agents-shared/draft-shell";
import type { DraftInstance } from "../stays-on-agents-shared/fixtures";
import { StaysOnRow, type StaysOnRowProps } from "../stays-on-agents-shared/stays-on-row";
import { SPAWNED_BY } from "../stays-on-agents-shared/sessions-fixtures";

function DraftBoardCard({ actions: _actions, ...props }: SessionBoardCardProps) {
  const creator = SPAWNED_BY[props.session.id];
  return (
    <div className="relative">
      <SessionBoardCardView {...props} runtime={boardRuntimeFixture(props.session)} />
      {creator ? (
        <span className="absolute right-10 bottom-3 rounded-full bg-card pl-1">
          <AgentAvatar
            appearance={creator.agent.appearance}
            label={`Started by ${creator.agent.label}`}
            quietIdle
            size={16}
            surfaceClassName="ring-card"
          />
        </span>
      ) : null}
    </div>
  );
}

export interface KanbanLane {
  name: string;
  instances?: readonly DraftInstance[];
  columns: readonly SessionsBoardColumnView[];
  count: number;
  /** Agents seated before `+N`; the board header has room for a full crew. */
  limit?: number;
  rowProps?: Partial<StaysOnRowProps>;
}

function withoutChildren(columns: readonly SessionsBoardColumnView[]) {
  return columns.map(column => {
    const sessions = column.sessions.filter(session => !SPAWNED_BY[session.id]);
    return { ...column, sessions, count: sessions.length };
  });
}

function Lane({
  lane,
  hideChildren,
  collapsed,
  onCollapse,
}: {
  lane: KanbanLane;
  hideChildren: boolean;
  collapsed: Record<string, boolean>;
  onCollapse: (id: string, next: boolean) => void;
}) {
  return (
    <section aria-label={lane.name} className={SESSIONS_BOARD_LANE}>
      <header className={cn(SESSIONS_BOARD_LANE_HEADER, "min-h-11 gap-3 py-1")}>
        <WorkspaceBadge identity={{ kind: "project", name: lane.name }} variant="chrome" />
        <span className={SESSIONS_BOARD_LANE_COUNT}>{lane.count}</span>
        <StaysOnRow
          instances={lane.instances ?? []}
          layout="band"
          limit={lane.limit ?? 12}
          scopeName={lane.name}
          surface="ring-canvas"
          {...lane.rowProps}
        />
      </header>
      <SessionsBoardColumnsRow
        collapsedColumns={collapsed}
        columns={hideChildren ? withoutChildren(lane.columns) : [...lane.columns]}
        currentSessionId={null}
        layout="lane"
        onColumnCollapsedChange={onCollapse}
        onOpen={() => undefined}
        regionPrefix={lane.name}
        testIdPrefix={`draft-lane-${lane.name}`}
        workspaceNameOf={() => null}
      />
    </section>
  );
}

export function KanbanWindow({
  lanes,
  top,
  initialHide = false,
}: {
  lanes: readonly KanbanLane[];
  top?: readonly DraftInstance[];
  initialHide?: boolean;
}) {
  const [hide, setHide] = useState(initialHide);
  const [view, setView] = useState<"list" | "kanban">("kanban");
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({ done: true });
  const topbar: TopbarSlotValue = {
    crumb: "Sessions",
    overflow: <SessionViewToggle onValueChange={setView} value={view} />,
    actions: (
      <>
        <Button
          aria-pressed={hide}
          onClick={() => setHide(!hide)}
          size="sm"
          type="button"
          variant="quiet"
        >
          <ListTree aria-hidden="true" data-icon="inline-start" />
          Hide child sessions
        </Button>
        <Button size="sm" type="button" variant="secondary">
          <Plus aria-hidden="true" data-icon="inline-start" />
          New session
        </Button>
      </>
    ),
  };
  const globalRow: ReactNode =
    top && top.length > 0 ? (
      <StaysOnRow
        className="px-1"
        instances={top}
        label={<Eyebrow className="pr-1.5 text-faint">Global</Eyebrow>}
        layout="band"
        scopeName="Global"
        surface="ring-canvas"
      />
    ) : null;
  return (
    <DraftShell app="sessions" glyph={<SquareKanban />} title="Sessions" topbar={topbar}>
      <SessionBoardCardSlot value={DraftBoardCard}>
        <div className={SESSIONS_BOARD_FRAME}>
          <div className={SESSIONS_BOARD_SCROLLER}>
            <div className={SESSIONS_BOARD_LANES}>
              {globalRow}
              {lanes.map(lane => (
                <Lane
                  collapsed={collapsed}
                  hideChildren={hide}
                  key={lane.name}
                  lane={lane}
                  onCollapse={(id, next) => setCollapsed(value => ({ ...value, [id]: next }))}
                />
              ))}
            </div>
          </div>
        </div>
      </SessionBoardCardSlot>
    </DraftShell>
  );
}
