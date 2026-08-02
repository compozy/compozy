import { Link } from "@tanstack/react-router";
import { ChevronRight } from "lucide-react";

import { cn, formatRelativeTime } from "@compozy/ui";

import { loopRunBestLabel } from "../../lib/loop-generation-presentation";
import type { LoopRun } from "../../types";
import { LoopStatusPill } from "../loop-status-pill";

interface LoopRecentRunsProps {
  runs: readonly LoopRun[];
}

const recentRunRowClassName = cn(
  "grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1.5 border-t border-line-soft px-4 py-2.5 transition-colors first:border-t-0 hover:bg-row-hover",
  "min-[720px]:grid-cols-[120px_96px_minmax(0,1fr)_72px_112px_64px_16px] min-[720px]:gap-3.5"
);

/** Trigger line for a run row (`cli · operator`, `schedule`). */
function triggerLine(run: LoopRun): string {
  const kind = run.started_origin_kind || run.started_by_kind || "";
  const ref = run.started_by_ref || run.started_origin_ref || "";
  if (kind && ref) return `${kind} · ${ref}`;
  return kind || ref || "—";
}

/** Recent runs of one Loop as a compact row list; each row opens the run detail. */
export function LoopRecentRuns({ runs }: LoopRecentRunsProps) {
  if (runs.length === 0) {
    return (
      <div
        className="rounded-lg border border-line bg-canvas-soft px-4 py-6 text-center text-small-body text-subtle"
        data-testid="loop-recent-runs-empty"
      >
        This Loop has not run yet.
      </div>
    );
  }
  return (
    <div
      className="flex flex-col rounded-lg border border-line bg-canvas-soft"
      data-testid="loop-recent-runs"
    >
      {runs.map(run => (
        <Link
          key={run.id}
          to="/loop-runs/$runId"
          params={{ runId: run.id }}
          className={recentRunRowClassName}
          data-testid="loop-recent-run-row"
        >
          <LoopStatusPill
            className="col-start-1 row-start-1 min-[720px]:col-auto min-[720px]:row-auto"
            status={run.status}
          />
          <span className="col-start-1 row-start-2 min-w-0 truncate font-mono text-mono-id text-fg min-[720px]:col-auto min-[720px]:row-auto">
            {run.id}
          </span>
          <span className="col-span-2 row-start-3 min-w-0 truncate text-form-hint text-subtle min-[720px]:col-auto min-[720px]:row-auto">
            {triggerLine(run)}
          </span>
          <span className="col-start-1 row-start-4 text-xs tabular-nums text-muted min-[720px]:col-auto min-[720px]:row-auto">
            {run.generation} gens
          </span>
          <span
            className="col-start-2 row-start-4 text-right font-mono text-mono-id tabular-nums text-muted min-[720px]:col-auto min-[720px]:row-auto"
            data-testid="loop-recent-run-best"
          >
            {loopRunBestLabel(run) ?? "—"}
          </span>
          <span className="col-start-2 row-start-1 justify-self-end text-right text-form-hint tabular-nums text-faint min-[720px]:col-auto min-[720px]:row-auto">
            {formatRelativeTime(run.created_at)}
          </span>
          <span className="col-start-2 row-start-2 justify-self-end text-faint min-[720px]:col-auto min-[720px]:row-auto">
            <ChevronRight aria-hidden="true" className="size-3.5" />
          </span>
        </Link>
      ))}
    </div>
  );
}
