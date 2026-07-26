import { Link } from "@tanstack/react-router";
import { ChevronRight } from "lucide-react";

import { formatRelativeTime } from "@agh/ui";

import type { LoopRun } from "../../types";
import { LoopStatusPill } from "../loop-status-pill";

interface LoopRecentRunsProps {
  runs: readonly LoopRun[];
}

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
          className="grid grid-cols-[120px_96px_minmax(0,1fr)_72px_64px_16px] items-center gap-3.5 border-t border-line-soft px-4 py-2.5 transition-colors first:border-t-0 hover:bg-row-hover"
          data-testid="loop-recent-run-row"
        >
          <LoopStatusPill status={run.status} />
          <span className="font-mono text-mono-id text-fg">{run.id}</span>
          <span className="truncate text-form-hint text-subtle">{triggerLine(run)}</span>
          <span className="text-xs tabular-nums text-muted">{run.generation} gens</span>
          <span className="text-right text-form-hint tabular-nums text-faint">
            {formatRelativeTime(run.created_at)}
          </span>
          <span className="justify-self-end text-faint">
            <ChevronRight aria-hidden="true" className="size-3.5" />
          </span>
        </Link>
      ))}
    </div>
  );
}
