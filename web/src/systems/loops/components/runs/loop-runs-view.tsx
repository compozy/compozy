import type { ComponentProps, ReactNode } from "react";
import { GitBranch, PlugZap, RotateCcw, TriangleAlert } from "lucide-react";

import {
  Alert,
  AlertActions,
  AlertDescription,
  Button,
  cn,
  Empty,
  formatRelativeTime,
  Skeleton,
  SkeletonRows,
} from "@compozy/ui";

import { buildRunsRoster, type LoopOutcomeValue } from "../../lib/loop-runs-view";
import type { LoopRun } from "../../types";
import { LoopRunsTable } from "./loop-runs-table";

export interface LoopRunsViewProps extends Omit<ComponentProps<"div">, "children"> {
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
  /**
   * When the rows below were last read, as an ISO timestamp.
   *
   * A degraded transport that says "the list below is the last read" without
   * saying how old it is leaves the reader to assume it is current — which is
   * the one thing the truthful-UI rule forbids a stale view from doing.
   */
  lastReadAt?: string;
}

/** Which transport failed. They are told apart because they recover differently. */
type DegradedCause = "read-failed" | "reconnecting";

/**
 * A degraded transport is not an empty workspace, so it never borrows the empty
 * state's words. It names the cause, says what the rows below actually are, and
 * offers a re-read. Polite live region: the daemon reconnecting is not an alarm.
 *
 * A failed list read and a dropped stream are different sentences: the first is
 * a request that came back an error and `Retry now` re-issues it; the second is
 * a subscription the client is already re-opening on its own. Printing
 * "reconnecting" over a failed read tells the reader to wait for something that
 * is not going to happen.
 */
function DegradedNotice({
  cause,
  hasRows,
  lastReadAt,
  onRetry,
}: {
  cause: DegradedCause;
  hasRows: boolean;
  lastReadAt?: string;
  onRetry?: () => void;
}) {
  const failed = cause === "read-failed";
  // Only ages a read that is actually on screen; "no runs read yet" has no age.
  const age = hasRows && lastReadAt ? formatRelativeTime(lastReadAt) : null;
  const retained = age ? `, from ${age}` : "";
  const body = failed
    ? hasRows
      ? `This workspace's runs could not be read. The list below is the last read that worked${retained}.`
      : "This workspace's runs could not be read."
    : hasRows
      ? `Reconnecting to the daemon. The list below is the last read${retained}.`
      : "Reconnecting to the daemon. No runs have been read yet.";
  return (
    <Alert
      data-cause={cause}
      data-testid="loop-runs-degraded"
      role="status"
      variant={failed ? "danger" : "neutral"}
    >
      {failed ? <TriangleAlert aria-hidden="true" /> : <PlugZap aria-hidden="true" />}
      <AlertDescription>{body}</AlertDescription>
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
  lastReadAt,
  className,
  ...props
}: LoopRunsViewProps) {
  const roster = buildRunsRoster(runs, outcome);
  // A failed read outranks a reconnect: it is the more specific fact, and it is
  // the one the reader can act on.
  const cause: DegradedCause | null = isError
    ? "read-failed"
    : isReconnecting
      ? "reconnecting"
      : null;
  const degraded = cause !== null;
  const hasRows = roster.groups.length > 0;
  return (
    <div
      className={cn("flex flex-col gap-2.5", className)}
      data-needs-you-count={roster.needsYouCount}
      data-testid="loop-runs-view"
      data-loaded-count={roster.loadedCount}
      {...props}
    >
      {cause ? (
        <DegradedNotice cause={cause} hasRows={hasRows} lastReadAt={lastReadAt} onRetry={onRetry} />
      ) : null}
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
