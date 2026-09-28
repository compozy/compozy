import { Link } from "@tanstack/react-router";
import { TriangleAlert } from "lucide-react";

import { cn, Pill, PillDot, TableCell, TableRow, Time } from "@compozy/ui";

import { formatClockDuration, runElapsedSeconds } from "../../lib/loop-run-usage";
import type { LoopRunRow as LoopRunRowModel } from "../../lib/loop-runs-view";
import { ProfileOwnerTag, type ProfileOwner } from "@/systems/profiles";

interface LoopRunRowProps {
  row: LoopRunRowModel;
  /** The run's profile, supplied only in aggregate mode. */
  owner?: ProfileOwner;
  /** Display clock, so a live run's duration ticks the same as on the Loop page. */
  nowMs: number;
}

const META_CELL = "font-mono text-mono-id tabular-nums text-muted";

/**
 * One roster row: Loop · Status · Progress · Started · Duration.
 *
 * Everything the row says comes from the server-owned row model — it never
 * re-derives a status, an attention marker, or a step count from the raw run.
 * Spend (generations, best score, budget) is deliberately absent: it is demoted
 * to the run page, where there is room to say what it means.
 */
export function LoopRunRow({ row, owner, nowMs }: LoopRunRowProps) {
  const { run } = row;
  return (
    <TableRow
      className={cn(row.needsYou && "bg-row-selected hover:bg-surface-glaze")}
      data-needs-you={row.needsYou ? "true" : undefined}
      data-run-id={run.id}
      data-status={run.status}
      data-testid="loop-run-row"
    >
      <TableCell className="w-full max-w-0 py-2.5">
        <span className="flex min-w-0 flex-col gap-0.5">
          <span className="flex min-w-0 items-center gap-2">
            <Link
              className="min-w-0 truncate text-ws-name font-medium text-fg-strong underline-offset-3 hover:underline"
              data-testid="loop-run-name"
              params={{ runId: run.id }}
              title={run.id}
              to="/loop-runs/$runId"
            >
              {run.loop_name}
            </Link>
            {owner ? <ProfileOwnerTag className="shrink-0" owner={owner} /> : null}
          </span>
          {row.summaryLine ? (
            <span className="truncate text-small-body text-subtle" data-testid="loop-run-summary">
              {row.summaryLine}
            </span>
          ) : null}
        </span>
      </TableCell>
      <TableCell>
        <Pill data-testid="loop-run-status" tone={row.statusTone}>
          {/* The needs-you chip carries a glyph as well as tone, so colour never
              travels alone on the one row a person has to act on. Every other
              status keeps production's dot-and-label chip vocabulary. */}
          {row.needsYou ? (
            <TriangleAlert aria-hidden="true" />
          ) : (
            <PillDot pulse={row.statusPulse} />
          )}
          {row.statusLabel}
        </Pill>
      </TableCell>
      <TableCell className={META_CELL} data-testid="loop-run-progress">
        {row.progressLabel}
      </TableCell>
      <TableCell className={META_CELL} data-testid="loop-run-started">
        <Time iso={run.created_at} />
      </TableCell>
      <TableCell className={META_CELL} data-testid="loop-run-duration">
        {formatClockDuration(runElapsedSeconds(run, nowMs))}
      </TableCell>
    </TableRow>
  );
}
