import { Plus, SquareKanban } from "@compozy/ui/icons";
import { useMemo } from "react";

import { Button, type TopbarSlotValue } from "@compozy/ui";

import { ComputerMetaLabel } from "@/systems/computers/components/computer-chip";
import type { RunOnTarget } from "@/systems/computers/components/run-on-picker";
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
  SESSIONS_BOARD_SCROLLER,
} from "@/systems/session/components/session-board/sessions-board-frame";
import type { SessionsBoardColumnView } from "@/systems/session/hooks/use-sessions-board";
import type { SessionBoardColumnId } from "@/systems/session/lib/session-board-contract";
import { boardRuntimeFixture } from "@/systems/session/mocks/sessions-board-view-fixtures";

import { DraftShell } from "../always-on-agents-shared/draft-shell";
import { RunOnSessionCreateDialog } from "./run-on-session-create";

const ENTRY_BY_ID = new Map<string, RemoteSessionRowEntry>(
  REMOTE_SESSION_ENTRIES.map(entry => [entry.id, entry])
);

function KanbanBackdropCard({ actions: _actions, ...props }: SessionBoardCardProps) {
  const meta = ENTRY_BY_ID.get(props.session.id);
  const offline = Boolean(meta?.computerOffline);
  return (
    <div className={offline ? "opacity-60" : undefined}>
      <SessionBoardCardView
        {...props}
        metaExtra={
          meta && !meta.isLocal ? (
            <ComputerMetaLabel
              className="ml-1"
              iconKind={meta.iconKind}
              name={meta.computerName}
              offline={offline}
              platform={meta.platform}
            />
          ) : undefined
        }
        runtime={boardRuntimeFixture(props.session)}
      />
    </div>
  );
}

const COLUMN_DEFS: ReadonlyArray<{ id: SessionBoardColumnId; label: string }> = [
  { id: "needs_you", label: "Needs you" },
  { id: "working", label: "Working" },
  { id: "in_review", label: "In review" },
  { id: "done", label: "Done" },
];

function buildColumns(): SessionsBoardColumnView[] {
  const payloads = REMOTE_SESSION_ENTRIES.map(toBoardSessionPayload);
  return COLUMN_DEFS.map(def => {
    const sessions = payloads.filter(session => session.board_column === def.id);
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

/**
 * Screen 05 Host B: Kanban "New session" opens the real production `SessionCreateDialog`
 * (`session-board-window.tsx`) with `Run on` before Agent and Project (`_uiux.md` U4).
 */
export function RunOnKanbanNewScene({
  initialTarget = "cmp_1c9d0a2b",
  pickerDefaultOpen = false,
}: {
  initialTarget?: RunOnTarget;
  pickerDefaultOpen?: boolean;
}) {
  const columns = useMemo(() => buildColumns(), []);

  const topbar: TopbarSlotValue = {
    crumb: "Sessions",
    overflow: <SessionViewToggle onValueChange={() => undefined} value="kanban" />,
    actions: (
      <Button size="sm" type="button" variant="secondary">
        <Plus aria-hidden="true" data-icon="inline-start" />
        New session
      </Button>
    ),
  };

  const overlay = (
    <RunOnSessionCreateDialog
      initialProject="api"
      initialTarget={initialTarget}
      loadBalancingEnabled
      pickerDefaultOpen={pickerDefaultOpen}
    />
  );

  return (
    <DraftShell
      app="sessions"
      glyph={<SquareKanban />}
      overlay={overlay}
      scope="All projects"
      title="Sessions"
      topbar={topbar}
    >
      <SessionBoardCardSlot value={KanbanBackdropCard}>
        <div className={SESSIONS_BOARD_FRAME}>
          <div className={SESSIONS_BOARD_SCROLLER}>
            <SessionsBoardColumnsRow
              collapsedColumns={{}}
              columns={columns}
              currentSessionId={null}
              layout="fill"
              onColumnCollapsedChange={() => undefined}
              onOpen={() => undefined}
              workspaceNameOf={session => ENTRY_BY_ID.get(session.id)?.project ?? null}
            />
          </div>
        </div>
      </SessionBoardCardSlot>
    </DraftShell>
  );
}
