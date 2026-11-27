import { ChevronRight } from "@compozy/ui/icons";

import { Icon, StatusDot, cn } from "@compozy/ui";

import { useOptionalUpgradeSheetActions } from "@/systems/account";
import {
  ComputerPlatformIcon,
  ScopedComputerError,
  ScopedComputerSkeleton,
  isComputerCounted,
} from "@/systems/computers";

import { useSessionsBoardColumns } from "../../hooks/use-sessions-board";
import type { SessionsBoardComputerBand } from "../../hooks/use-sessions-board-computers";
import { SESSION_BOARD_COLUMNS, type SessionBoardColumnId, type SessionBoardPayload } from "../../lib/session-board-contract";
import { AlwaysOnListTop } from "../always-on/always-on-list-top";
import { SessionListComputerStateNotice } from "../session-list/session-list-computer-note";
import { SessionsBoardColumnGlyph } from "./sessions-board-column-glyph";
import { SessionsBoardColumnsRow } from "./sessions-board-columns-row";
import { SESSIONS_BOARD_LANE_COUNT } from "./sessions-board-frame";
import { SessionsBoardSwimlane } from "./sessions-board-swimlane";
import type { SessionsBoardRow } from "./sessions-board-view";

/** A folded band's one line: all five columns' mark, name and exact facet count (`K3`, `K5`). */
function FoldedBoardBandCounts({
  boardCounts,
  needsYou,
  testIdPrefix,
}: {
  boardCounts: Record<SessionBoardColumnId, number> | undefined;
  needsYou: number;
  testIdPrefix: string;
}) {
  if (!boardCounts) return null;
  return (
    <div
      className="flex flex-wrap items-center gap-x-5 gap-y-1 pl-8 text-small-body text-muted-foreground"
      data-testid={`${testIdPrefix}-folded-counts`}
    >
      {needsYou > 0 ? (
        <StatusDot
          label={`${needsYou} need${needsYou === 1 ? "s" : ""} you`}
          size="sm"
          tone="attn"
        />
      ) : null}
      {SESSION_BOARD_COLUMNS.map(column => (
        <span className="flex items-center gap-1.5" key={column.id}>
          <SessionsBoardColumnGlyph id={column.id} />
          {column.label}
          <span className="text-subtle tabular-nums">{boardCounts[column.id] ?? 0}</span>
        </span>
      ))}
    </div>
  );
}

/** Mounted only for an open ungrouped band so folded bands never read column pages (`C7`, `K3`). */
function BandColumnsRow({
  band,
  row,
  rank,
}: {
  band: SessionsBoardComputerBand;
  row: SessionsBoardRow;
  rank: number;
}) {
  const columns = useSessionsBoardColumns(
    band.baseFilters,
    band.source.read === "ok" && !band.emptyRemoteMatch,
    band.source.key
  );
  return (
    <SessionsBoardColumnsRow
      {...row}
      columns={columns}
      layout="lane"
      regionPrefix={band.source.name}
      selectionRow={{ key: `band:${band.source.key}`, rank }}
      testIdPrefix={`sessions-board-band-${band.source.key}`}
      workspaceNameOf={band.workspaceNameOf}
    />
  );
}

function BandBody({
  band,
  row,
  rank,
  focusedWorkspaceId,
}: {
  band: SessionsBoardComputerBand;
  row: SessionsBoardRow;
  rank: number;
  focusedWorkspaceId?: string | null;
}) {
  const upgradeSheet = useOptionalUpgradeSheetActions();
  const { source } = band;
  const testIdPrefix = `sessions-board-band-${source.key}`;

  if (source.read === "connecting") {
    return (
      <ScopedComputerSkeleton
        className="ml-8 max-w-md"
        computerId={source.key}
        computerName={source.name}
      />
    );
  }
  if (source.read === "failed") {
    return (
      <ScopedComputerError
        className="ml-8 w-max"
        computerId={source.key}
        computerName={source.name}
        onRetry={source.retry}
      />
    );
  }
  if (source.read !== "ok" && source.read !== "offline") {
    return (
      <div className="pl-6">
        <SessionListComputerStateNotice
          computerId={source.key}
          name={source.name}
          onRetry={source.retry}
          onUpgrade={upgradeSheet ? () => upgradeSheet.openUpgradeSheet() : undefined}
          read={source.read}
          testIdPrefix={testIdPrefix}
        />
      </div>
    );
  }
  if (source.read === "ok" && source.count === 0) {
    return (
      <p className="m-0 pl-8 text-small-body text-muted-foreground">
        No sessions on {source.name} yet.
      </p>
    );
  }

  const dim = source.read === "offline" ? "opacity-70" : undefined;
  if (band.lanes !== null) {
    return (
      <div className={cn("flex flex-col gap-1 pl-6", dim)}>
        {band.lanes.map((lane, index) => (
          <SessionsBoardSwimlane
            {...row}
            key={lane.group.key}
            lane={lane}
            selectionRow={{
              key: `band:${source.key}:lane:${lane.group.key}`,
              rank: rank * 100 + index,
            }}
            workspaceNameOf={band.workspaceNameOf}
          />
        ))}
      </div>
    );
  }
  return (
    <div className={cn("flex flex-col gap-2 pl-6", dim)}>
      {source.isLocal ? (
        <AlwaysOnListTop
          currentSessionId={row.currentSessionId}
          groupBy="none"
          onOpenSession={session => row.onOpen(session as SessionBoardPayload)}
          surface="ring-canvas"
          workspaceId={focusedWorkspaceId ?? null}
        />
      ) : null}
      <BandColumnsRow band={band} rank={rank} row={row} />
    </div>
  );
}

/** One computer band on the Separate Kanban board (`F13`, `K2`, `K3`, `K5`, `K6`). */
export function SessionsBoardComputerBandView({
  band,
  row,
  rank,
  focusedWorkspaceId,
}: {
  band: SessionsBoardComputerBand;
  row: SessionsBoardRow;
  rank: number;
  focusedWorkspaceId?: string | null;
}) {
  const { source, collapsed, onToggle } = band;
  const counted = isComputerCounted(source.read);
  const testId = `sessions-board-band-${source.key}`;

  return (
    <section
      aria-label={source.name}
      className="flex flex-col gap-2 border-t border-line-soft pt-3 first:border-t-0 first:pt-0"
      data-collapsed={collapsed ? "true" : undefined}
      data-testid={testId}
    >
      <button
        aria-expanded={!collapsed}
        className="flex h-8 w-max items-center gap-2 rounded-sm px-1 hover:bg-surface-2 focus-visible:shadow-focus-ring focus-visible:outline-none"
        data-testid={`${testId}-header`}
        onClick={onToggle}
        type="button"
      >
        <Icon
          as={ChevronRight}
          className={cn("shrink-0 text-faint transition-transform", !collapsed && "rotate-90")}
          size="sm"
        />
        <ComputerPlatformIcon
          className="size-4 shrink-0"
          iconKind={source.computer.iconKind}
          platform={source.computer.platform}
          size="sm"
          tone="muted"
        />
        <span className="text-item-title font-medium text-fg">{source.name}</span>
        <span
          className={cn(
            "text-eyebrow",
            source.read === "incompatible"
              ? "text-warning"
              : source.read === "offline" || source.read === "offline_empty"
                ? "text-faint"
                : "text-subtle"
          )}
        >
          {source.detail}
        </span>
        {counted && source.count !== null ? (
          <span className={SESSIONS_BOARD_LANE_COUNT} data-testid={`${testId}-count`}>
            {source.count}
          </span>
        ) : null}
      </button>
      {collapsed ? (
        counted ? (
          <FoldedBoardBandCounts
            boardCounts={band.boardCounts}
            needsYou={source.needsYou}
            testIdPrefix={testId}
          />
        ) : null
      ) : (
        <BandBody band={band} focusedWorkspaceId={focusedWorkspaceId} rank={rank} row={row} />
      )}
    </section>
  );
}
