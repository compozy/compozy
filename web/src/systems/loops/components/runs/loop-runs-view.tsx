import type { ReactNode } from "react";
import { GitBranch, PlugZap, RotateCcw } from "lucide-react";

import {
  Alert,
  AlertActions,
  AlertDescription,
  Button,
  Empty,
  Skeleton,
  SkeletonRows,
} from "@compozy/ui";

import { buildRunsRoster, type LoopOutcomeValue } from "../../lib/loop-runs-view";
import type { LoopRun } from "../../types";
import { LoopRunsTable } from "./loop-runs-table";

export interface LoopRunsViewProps {
  runs: readonly LoopRun[];
  /** Outcome filter driven by the toolbar chip bar. */
  outcome: LoopOutcomeValue;
  /** The list read failed. Any rows below are the last good read, not fresh truth. */
  isError?: boolean;
  /** The run stream dropped and is retrying. Same rule: what shows is the last read. */
  isReconnecting?: boolean;
  /** Re-reads the roster. Omit it and the degraded notice states the cause only. */
  onRetry?: () => void;
  /** Runs the empty state's action (`Browse loops` / `Clear filter`). */
  onEmptyAction?: () => void;
}

/**
 * A degraded transport is not an empty workspace, so it never borrows the empty
 * state's words. It names the cause, says what the rows below actually are, and
 * offers a re-read. Polite live region: the daemon reconnecting is not an alarm.
 */
function DegradedNotice({ hasRows, onRetry }: { hasRows: boolean; onRetry?: () => void }) {
  return (
    <Alert data-testid="loop-runs-degraded" role="status" variant="neutral">
      <PlugZap aria-hidden="true" />
      <AlertDescription>
        {hasRows
          ? "Reconnecting to the daemon. The list below is the last read."
          : "Reconnecting to the daemon. No runs have been read yet."}
      </AlertDescription>
      {onRetry ? (
        <AlertActions>
          <Button
            data-testid="loop-runs-degraded-retry"
            onClick={onRetry}
            size="sm"
            type="button"
            variant="outline"
          >
            <RotateCcw aria-hidden="true" />
            Retry now
          </Button>
        </AlertActions>
      ) : null}
    </Alert>
  );
}

/**
 * The workspace-wide runs roster, ranked needs-you first.
 *
 * Ordering is the daemon's — it ranks needs-you above live above terminal before
 * the page is cut — so this groups what arrives and never re-sorts it. There is
 * no KPI band at any scale: four counters above the list answered nothing the
 * groups do not already answer, and they pushed the runs that need a person
 * below the fold.
 */
export function LoopRunsView({
  runs,
  outcome,
  isError = false,
  isReconnecting = false,
  onRetry,
  onEmptyAction,
}: LoopRunsViewProps) {
  const roster = buildRunsRoster(runs, outcome);
  const degraded = isError || isReconnecting;
  const hasRows = roster.groups.length > 0;
  return (
    <div
      className="flex flex-col gap-2.5"
      data-needs-you-count={roster.needsYouCount}
      data-testid="loop-runs-view"
      data-total={roster.total}
    >
      {degraded ? <DegradedNotice hasRows={hasRows} onRetry={onRetry} /> : null}
      <RosterBody
        degraded={degraded}
        onEmptyAction={onEmptyAction}
        roster={roster}
        hasRows={hasRows}
      />
    </div>
  );
}

interface RosterBodyProps {
  roster: ReturnType<typeof buildRunsRoster>;
  hasRows: boolean;
  degraded: boolean;
  onEmptyAction?: () => void;
}

function RosterBody({ roster, hasRows, degraded, onEmptyAction }: RosterBodyProps): ReactNode {
  if (hasRows) {
    return (
      <div className="flex flex-col gap-3">
        {roster.groups.map(group => (
          <LoopRunsTable group={group} key={group.id} />
        ))}
      </div>
    );
  }
  // Rows have not arrived yet and the transport is degraded: keep the shape of
  // what is coming rather than claiming the workspace is empty.
  if (degraded) {
    return (
      <SkeletonRows
        aria-hidden="true"
        className="gap-3 rounded-lg border border-line bg-canvas-soft px-3.5 py-3"
        count={3}
        data-testid="loop-runs-skeleton"
      >
        <Skeleton className="h-3.5 w-1/2" />
      </SkeletonRows>
    );
  }
  if (!roster.emptyState) return null;
  return (
    <Empty
      action={
        onEmptyAction ? (
          <Button
            data-testid="loop-runs-empty-action"
            onClick={onEmptyAction}
            size="sm"
            type="button"
          >
            {roster.emptyState.actionLabel}
          </Button>
        ) : null
      }
      className="mx-auto my-8 max-w-md"
      data-testid="loop-runs-empty"
      description={roster.emptyState.body}
      icon={GitBranch}
      title={roster.emptyState.title}
    />
  );
}
