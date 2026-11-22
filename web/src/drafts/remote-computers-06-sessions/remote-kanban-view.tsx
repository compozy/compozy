import { Layers, Plus, SquareKanban } from "@compozy/ui/icons";
import { useMemo, useState } from "react";

import { Button, cn, Pill, type TopbarSlotValue } from "@compozy/ui";

import { ComputerMetaLabel } from "@/systems/computers/components/computer-chip";
import { ComputerPlatformIcon } from "@/systems/computers/components/computer-platform-icon";
import {
  REMOTE_SESSION_ENTRIES,
  toBoardSessionPayload,
  type RemoteSessionRowEntry,
} from "@/systems/computers/mocks/remote-computers-fixtures-sessions";
import type { SessionBoardCardProps } from "@/systems/session/components/session-board/session-board-card";
import { SessionBoardCardSlot } from "@/systems/session/components/session-board/session-board-card-slot";
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
import type { SessionBoardColumnId } from "@/systems/session/lib/session-board-contract";
import { boardRuntimeFixture } from "@/systems/session/mocks/sessions-board-view-fixtures";
import { WorkspaceBadge } from "@/systems/workspace/components/workspace-badge";

import { DraftShell } from "../always-on-agents-shared/draft-shell";

const COLUMN_DEFS: ReadonlyArray<{ id: SessionBoardColumnId; label: string }> = [
  { id: "needs_you", label: "Needs you" },
  { id: "working", label: "Working" },
  { id: "in_review", label: "In review" },
  { id: "done", label: "Done" },
];

function buildColumnsFor(entries: readonly RemoteSessionRowEntry[]): SessionsBoardColumnView[] {
  const payloads = entries.map(toBoardSessionPayload);
  return COLUMN_DEFS.map(def => {
    const sessions = payloads.filter(s => s.board_column === def.id);
    return {
      id: def.id,
      label: def.label,
      count: sessions.length,
      sessions,
      loading: false,
      failure: null,
      retrying: false,
      fetching: false,
      hasMore: false,
      loadingMore: false,
      loadMore: () => undefined,
      retry: () => undefined,
      total: sessions.length,
    };
  });
}

interface BoardSwimlane {
  key: string;
  project: string;
  computerName: string;
  isLocal: boolean;
  offline: boolean;
  iconKind: RemoteSessionRowEntry["iconKind"];
  platform: RemoteSessionRowEntry["platform"];
  entries: RemoteSessionRowEntry[];
}

export interface RemoteKanbanViewProps {
  sessions?: readonly RemoteSessionRowEntry[];
  initialGrouped?: boolean;
}

/**
 * Multi-computer Kanban board (`_uiux.md` U5):
 * - Unified columns with muted computer labels on remote cards and dimmed `Offline` cards
 * - Toggleable `project · computer` swimlanes keeping duplicate repo names separate per computer
 */
export function RemoteKanbanView({
  sessions = REMOTE_SESSION_ENTRIES,
  initialGrouped = false,
}: RemoteKanbanViewProps) {
  const [grouped, setGrouped] = useState(initialGrouped);
  const [collapsedColumns, setCollapsedColumns] = useState<Record<string, boolean>>({});
  const [selectedId, setSelectedId] = useState<string | null>("sess-devbox-1");

  const entryMap = useMemo(() => new Map(sessions.map(entry => [entry.id, entry])), [sessions]);

  const flatColumns = useMemo(() => buildColumnsFor(sessions), [sessions]);

  const swimlanes = useMemo<BoardSwimlane[]>(() => {
    const map = new Map<string, BoardSwimlane>();
    for (const item of sessions) {
      const key = `${item.project}::${item.computerId}`;
      const existing = map.get(key);
      if (existing) {
        existing.entries.push(item);
      } else {
        map.set(key, {
          key,
          project: item.project,
          computerName: item.isLocal ? "This computer" : item.computerName,
          isLocal: item.isLocal,
          offline: Boolean(item.computerOffline),
          iconKind: item.iconKind,
          platform: item.platform,
          entries: [item],
        });
      }
    }
    return [...map.values()].sort((a, b) => {
      const p = a.project.localeCompare(b.project);
      if (p !== 0) return p;
      if (a.isLocal !== b.isLocal) return a.isLocal ? -1 : 1;
      return a.computerName.localeCompare(b.computerName);
    });
  }, [sessions]);

  function RemoteBoardCard({ actions: _actions, ...props }: SessionBoardCardProps) {
    const meta = entryMap.get(props.session.id);
    const offline = Boolean(meta?.computerOffline);
    return (
      <div className={cn(offline && "opacity-60")}>
        <SessionBoardCardView
          {...props}
          metaExtra={
            meta && !meta.isLocal && !grouped ? (
              <ComputerMetaLabel
                className="ml-1"
                iconKind={meta.iconKind}
                name={meta.computerName}
                offline={offline}
                platform={meta.platform}
              />
            ) : offline && grouped ? (
              <Pill className="ml-1" form="tint" size="sm" tone="neutral">
                Offline
              </Pill>
            ) : undefined
          }
          runtime={boardRuntimeFixture(props.session)}
        />
      </div>
    );
  }

  const topbar: TopbarSlotValue = {
    crumb: "Sessions",
    overflow: <SessionViewToggle onValueChange={() => undefined} value="kanban" />,
    actions: (
      <>
        <Button
          aria-pressed={grouped}
          onClick={() => setGrouped(value => !value)}
          size="sm"
          type="button"
          variant={grouped ? "secondary" : "quiet"}
        >
          <Layers aria-hidden="true" data-icon="inline-start" />
          {grouped ? "Grouped by project · computer" : "Group by project"}
        </Button>
        <Button size="sm" type="button" variant="secondary">
          <Plus aria-hidden="true" data-icon="inline-start" />
          New session
        </Button>
      </>
    ),
  };

  return (
    <DraftShell
      app="sessions"
      glyph={<SquareKanban />}
      scope="All projects"
      sessionBadge={2}
      title="Sessions"
      topbar={topbar}
    >
      <SessionBoardCardSlot value={RemoteBoardCard}>
        <div className={SESSIONS_BOARD_FRAME}>
          <div className={SESSIONS_BOARD_SCROLLER}>
            {!grouped ? (
              <SessionsBoardColumnsRow
                collapsedColumns={collapsedColumns}
                columns={flatColumns}
                currentSessionId={selectedId}
                layout="fill"
                onColumnCollapsedChange={(id, next) =>
                  setCollapsedColumns(prev => ({ ...prev, [id]: next }))
                }
                onOpen={session => setSelectedId(session.id)}
                workspaceNameOf={session => entryMap.get(session.id)?.project ?? null}
              />
            ) : (
              <div className={SESSIONS_BOARD_LANES}>
                {swimlanes.map(lane => (
                  <section
                    aria-label={`${lane.project} · ${lane.computerName}`}
                    className={cn(SESSIONS_BOARD_LANE, lane.offline && "opacity-70")}
                    key={lane.key}
                  >
                    <header className={cn(SESSIONS_BOARD_LANE_HEADER, "min-h-9 gap-2 py-1")}>
                      <WorkspaceBadge
                        identity={{ kind: "project", name: lane.project }}
                        variant="chrome"
                      />
                      <span aria-hidden="true" className="text-faint">
                        ·
                      </span>
                      <span className="inline-flex items-center gap-1.5 text-small-body text-muted-foreground">
                        {!lane.isLocal ? (
                          <ComputerPlatformIcon
                            className="size-3.5 shrink-0"
                            iconKind={lane.iconKind}
                            platform={lane.platform}
                            size="sm"
                            tone="muted"
                          />
                        ) : null}
                        <span>{lane.computerName}</span>
                      </span>
                      {lane.offline ? (
                        <Pill form="tint" size="sm" tone="neutral">
                          Offline
                        </Pill>
                      ) : null}
                      <span className={SESSIONS_BOARD_LANE_COUNT}>{lane.entries.length}</span>
                    </header>
                    <SessionsBoardColumnsRow
                      collapsedColumns={collapsedColumns}
                      columns={buildColumnsFor(lane.entries)}
                      currentSessionId={selectedId}
                      layout="lane"
                      onColumnCollapsedChange={(id, next) =>
                        setCollapsedColumns(prev => ({ ...prev, [id]: next }))
                      }
                      onOpen={session => setSelectedId(session.id)}
                      regionPrefix={`${lane.project} · ${lane.computerName}`}
                      testIdPrefix={`remote-lane-${lane.project}-${lane.computerName}`}
                      workspaceNameOf={() => null}
                    />
                  </section>
                ))}
              </div>
            )}
          </div>
        </div>
      </SessionBoardCardSlot>
    </DraftShell>
  );
}
