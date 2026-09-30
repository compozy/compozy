import { Link } from "@tanstack/react-router";
import { ChevronRight } from "lucide-react";

import { formatRelativeTime, Time } from "@compozy/ui";

import { useNowTick } from "../../hooks/use-now-tick";
import { loopStatusLabel } from "../../lib/loop-formatters";
import { formatClockDuration, runElapsedSeconds } from "../../lib/loop-run-usage";
import type { LoopRun } from "../../types";
import { LoopStatusPill } from "../loop-status-pill";

interface LoopRecentRunsProps {
  runs: readonly LoopRun[];
}

/**
 * Recent runs of one Loop as a compact row list: status, when it started, and how
 * long it ran. Run ids, rounds, and scores live on the run page each row opens.
 */
export function LoopRecentRuns({ runs }: LoopRecentRunsProps) {
  const nowMs = useNowTick(runs.some(run => run.status === "running"));

  if (runs.length === 0) {
    return (
      <div
        className="rounded-lg bg-sunken px-4 py-6 text-center text-small-body text-subtle"
        data-testid="loop-recent-runs-empty"
      >
        This Loop has not run yet.
      </div>
    );
  }
  return (
    <div className="flex flex-col rounded-lg bg-canvas shadow-card" data-testid="loop-recent-runs">
      {runs.map(run => (
        <Link
          key={run.id}
          aria-label={`${loopStatusLabel(run.status)} run, started ${formatRelativeTime(run.created_at)}`}
          to="/loop-runs/$runId"
          params={{ runId: run.id }}
          className="grid grid-cols-[auto_minmax(0,1fr)_auto_auto] items-center gap-3 border-t border-line-soft px-4 py-3 transition-colors first:border-t-0 hover:bg-row-hover"
          data-testid="loop-recent-run-row"
          title={run.id}
        >
          <LoopStatusPill status={run.status} />
          <span className="min-w-0 truncate text-form-hint text-subtle">
            <Time iso={run.created_at} />
          </span>
          <span
            className="text-right font-mono text-mono-id tabular-nums text-faint"
            data-testid="loop-recent-run-duration"
          >
            {formatClockDuration(runElapsedSeconds(run, nowMs))}
          </span>
          <ChevronRight aria-hidden="true" className="size-3.5 text-faint" />
        </Link>
      ))}
    </div>
  );
}
