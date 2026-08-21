import { ArrowUpRight, CornerDownRight } from "lucide-react";

import {
  Empty,
  PillGroup,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Time,
  cn,
} from "@compozy/ui";

import { formatClockDuration } from "../../../lib/loop-run-usage";
import type { LoopRosterRow, LoopRosterTableModel } from "../../../lib/loop-run-roster-table";
import { formatTokenCount } from "../../../lib/loop-runs-view";
import { LoopNodeStateChip } from "../loop-node-state-chip";

/** Sentinel for the "every round" option; rounds themselves are numbers. */
const ALL_ROUNDS = "all";

interface LoopNodeRosterProps {
  roster: LoopRosterTableModel;
  /** The round in view; `null` is every round, and then the round is shown per row. */
  round: number | null;
  onRoundChange: (round: number | null) => void;
  onSelect: (row: LoopRosterRow) => void;
  selectedKey: string | null;
}

/**
 * The complete roster: every node of every round, healthy included.
 *
 * The duration cell carries a micro-bar scaled against the run's longest node,
 * because "1m18s" tells you nothing about whether that is fast until you can see
 * it beside its siblings. A node that never started shows no bar at all rather
 * than an empty one, which would read as zero rather than as not-applicable.
 */
function LoopRosterDurationCell({ row }: { row: LoopRosterRow }) {
  if (row.progressState === "not-started" || row.durationMs === null) {
    return <span className="font-mono text-mono-id text-faint">not started</span>;
  }
  const clock = formatClockDuration(row.durationMs / 1000);
  return (
    <span className="flex flex-col gap-1">
      <span
        aria-hidden="true"
        className="h-1 w-full max-w-24 overflow-hidden rounded-pill bg-badge-fill"
      >
        <span
          className={cn(
            "block h-full rounded-pill",
            row.chip.tone === "danger" ? "bg-danger" : "bg-success"
          )}
          style={{ width: `${Math.round((row.durationRatio ?? 0) * 100)}%` }}
        />
      </span>
      <span className="font-mono text-mono-id text-subtle">
        {/* A step that started and has not ended is running, and the clock is
            how long it has been running — never "not started". */}
        {row.progressState === "in-progress" ? `${clock} · in progress` : clock}
      </span>
    </span>
  );
}

function LoopRosterUsageCell({ row }: { row: LoopRosterRow }) {
  if (row.usageTokens === null || row.usageTokens === 0) {
    return <span className="font-mono text-mono-id text-faint">—</span>;
  }
  return (
    <span className="font-mono text-mono-id text-subtle">
      {formatTokenCount(row.usageTokens)}
      {row.usageCostLabel ? <span className="text-faint"> · {row.usageCostLabel}</span> : null}
    </span>
  );
}

export function LoopNodeRoster({
  roster,
  round,
  onRoundChange,
  onSelect,
  selectedKey,
}: LoopNodeRosterProps) {
  if (roster.reachedNothing) {
    return (
      <Empty
        data-testid="loop-node-roster-empty"
        description="This run ended before it reached a single step, so there is nothing to list."
        title="No steps ran"
      />
    );
  }
  // Under one round the round is a constant and repeating it is noise. Across
  // rounds it is the only thing telling two rows of the same step apart.
  const showsRound = round === null && roster.rounds.length > 1;
  return (
    <div data-testid="loop-node-roster">
      {roster.rounds.length > 1 ? (
        <div className="flex items-center gap-2 border-b border-line-soft px-4 py-2.5">
          <PillGroup
            aria-label="Round"
            data-testid="loop-node-roster-round-filter"
            items={[
              ...roster.rounds.map(entry => ({ value: String(entry), label: `Round ${entry}` })),
              { value: ALL_ROUNDS, label: "All rounds" },
            ]}
            onChange={next => onRoundChange(next === ALL_ROUNDS ? null : Number(next))}
            size="sm"
            value={round === null ? ALL_ROUNDS : String(round)}
          />
        </div>
      ) : null}
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Step</TableHead>
            {showsRound ? <TableHead>Round</TableHead> : null}
            <TableHead>State</TableHead>
            <TableHead>Attempt</TableHead>
            <TableHead>Duration</TableHead>
            {/* Labelled once, in the header, so no row has to carry "est." */}
            <TableHead>Tokens · est. cost</TableHead>
            <TableHead>Session</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {roster.rows.map(row => (
            <TableRow
              className={cn(selectedKey === row.key && "bg-canvas-tint")}
              data-generation={row.generation}
              data-item-index={row.itemIndex}
              data-node-id={row.nodeId}
              data-state={row.chip.state}
              // Round, step and item: the same step id exists once per round and
              // once per fan-out worker, so anything shorter names several rows.
              data-testid={`loop-roster-row-${row.key}`}
              key={row.key}
              onClick={() => onSelect(row)}
            >
              <TableCell>
                <span className={cn("flex min-w-0 items-center gap-1.5", row.isBranch && "pl-4")}>
                  {row.isBranch ? (
                    <CornerDownRight aria-hidden="true" className="size-3 shrink-0 text-faint" />
                  ) : null}
                  <span className="min-w-0 truncate text-small-body font-medium text-fg-strong">
                    {row.nodeId}
                  </span>
                </span>
                {row.kindLabel ? (
                  <span className="mt-0.5 block font-mono text-mono-id text-faint">
                    {row.fanOutLabel ? `${row.kindLabel} · ${row.fanOutLabel}` : row.kindLabel}
                  </span>
                ) : null}
              </TableCell>
              {showsRound ? (
                <TableCell className="font-mono text-mono-id text-subtle">
                  Round {row.generation}
                </TableCell>
              ) : null}
              <TableCell>
                <LoopNodeStateChip chip={row.chip} />
              </TableCell>
              <TableCell className="font-mono text-mono-id text-subtle">
                {row.attemptLabel}
                {row.nextRetryAt ? (
                  <span className="text-faint">
                    {" · next "}
                    <Time iso={row.nextRetryAt} />
                  </span>
                ) : null}
              </TableCell>
              <TableCell>
                <LoopRosterDurationCell row={row} />
              </TableCell>
              <TableCell>
                <LoopRosterUsageCell row={row} />
              </TableCell>
              <TableCell>
                {row.sessionId ? (
                  <span className="inline-flex items-center gap-1 text-form-hint text-subtle">
                    Open
                    <ArrowUpRight aria-hidden="true" className="size-3" />
                  </span>
                ) : (
                  <span className="font-mono text-mono-id text-faint">—</span>
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
