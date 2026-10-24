import { Link } from "@tanstack/react-router";
import { ChevronRight } from "lucide-react";

import { formatRelativeTime, SkeletonText, Time } from "@compozy/ui";

import { useNowTick } from "../../hooks/use-now-tick";
import { placeholderSlots } from "../../lib/placeholder-slots";
import { loopStatusLabel } from "../../lib/loop-formatters";
import { formatClockDuration, runElapsedSeconds } from "../../lib/loop-run-usage";
import type { LoopRun } from "../../types";
import { LoopStatusMark } from "../loop-status-mark";
import { useWindowBindingSearch } from "@/systems/workspace";

// Shared by the real rows and their skeleton so a run row is the same height either way.
const RECENT_RUNS_CLASS = "flex flex-col rounded-lg bg-card shadow-card";
const RECENT_RUN_ROW_CLASS =
  "grid grid-cols-[auto_minmax(0,1fr)_auto_auto] items-center gap-3 border-t border-line-soft px-4 py-3 first:border-t-0";

interface LoopRecentRunsProps {
  runs: readonly LoopRun[];
}

/**
 * Recent runs of one Loop as a compact row list: status, when it started, and how
 * long it ran. Run ids, rounds, and scores live on the run page each row opens.
 */
export function LoopRecentRuns({ runs }: LoopRecentRunsProps) {
  const bindingSearch = useWindowBindingSearch();
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
    <div className={RECENT_RUNS_CLASS} data-testid="loop-recent-runs">
      {runs.map(run => (
        <Link
          key={run.id}
          aria-label={`${loopStatusLabel(run.status)} run, started ${formatRelativeTime(run.created_at)}`}
          search={bindingSearch}
          to="/loop-runs/$runId"
          params={{ runId: run.id }}
          className={`${RECENT_RUN_ROW_CLASS} transition-colors hover:bg-surface-2`}
          data-testid="loop-recent-run-row"
          title={run.id}
        >
          <LoopStatusMark status={run.status} />
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

/**
 * The recent runs while they are being read: `count` rows in the list's own card. Presentational,
 * so the caller wraps it in the `SkeletonRegion` for the area it stands in for.
 */
export function LoopRecentRunsSkeleton({ count = 3 }: { count?: number }) {
  return (
    <div aria-hidden="true" className={RECENT_RUNS_CLASS} data-testid="loop-recent-runs-loading">
      {placeholderSlots("recent-run", count).map(slot => (
        <div className={RECENT_RUN_ROW_CLASS} key={slot.id}>
          <SkeletonText className="w-16 text-small-body" />
          <SkeletonText className="w-24 text-form-hint" />
          <SkeletonText className="w-10 font-mono text-mono-id" />
          <ChevronRight aria-hidden="true" className="size-3.5 text-faint" />
        </div>
      ))}
    </div>
  );
}
