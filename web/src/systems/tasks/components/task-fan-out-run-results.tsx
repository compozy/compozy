import { CheckIcon, CircleAlertIcon, LoaderCircleIcon } from "lucide-react";

import {
  Button,
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemMedia,
  ItemTitle,
  MonoId,
} from "@compozy/ui";

import { WorktreeOriginSignal, type WorktreePayload } from "@/systems/workspace";

import type { FanOutTaskRunsResponse, TaskRun } from "../types";

type FanOutRun = FanOutTaskRunsResponse["runs"][number];

interface TaskFanOutRunResultsProps {
  runs: readonly FanOutRun[];
  /** Live task rows replace the accepted snapshots as materialization progresses. */
  liveRuns?: readonly TaskRun[];
  /** Used only to resolve a name for an id the response already attributed. */
  worktrees?: readonly WorktreePayload[];
  onRetry?: (runId: string) => void;
  retryPending?: boolean;
}

function isFailed(run: FanOutRun): boolean {
  return run.status === "failed" || run.status === "canceled" || Boolean(run.error);
}

function isCompleted(run: FanOutRun): boolean {
  return run.status === "completed";
}

/**
 * Per-run outcomes, attributed from the response and nothing else.
 *
 * A run without a worktree id is shown as that run id alone — a name is never
 * inferred from the isolation setting, and the row never invents a pending or
 * unavailable worktree.
 */
export function TaskFanOutRunResults({
  runs,
  liveRuns = [],
  worktrees,
  onRetry,
  retryPending = false,
}: TaskFanOutRunResultsProps) {
  if (runs.length === 0) return null;

  const liveById = new Map(liveRuns.map(run => [run.id, run]));
  const currentRuns = runs.map(run => liveById.get(run.id) ?? run);

  return (
    <div
      className="overflow-hidden rounded-lg border border-line bg-canvas-soft"
      data-slot="task-fan-out-run-results"
    >
      {currentRuns.map(run => {
        const failed = isFailed(run);
        const completed = isCompleted(run);
        const worktree = run.worktree_id
          ? worktrees?.find(entry => entry.id === run.worktree_id)
          : undefined;
        const attribution = worktree?.name ?? run.resolved_worktree_ref ?? run.worktree_id;
        return (
          <Item
            data-slot="task-fan-out-run-result"
            data-status={run.status}
            data-unattributed={attribution ? undefined : ""}
            key={run.id}
          >
            <ItemMedia
              className={failed ? "text-danger" : completed ? "text-success" : "text-info"}
            >
              {failed ? (
                <CircleAlertIcon aria-hidden="true" className="size-3" />
              ) : completed ? (
                <CheckIcon aria-hidden="true" className="size-3" />
              ) : (
                <LoaderCircleIcon aria-hidden="true" className="size-3" />
              )}
              <span className="sr-only">{run.status}</span>
            </ItemMedia>
            <ItemContent>
              <ItemTitle>{run.designation?.brief ?? run.id}</ItemTitle>
              <ItemDescription
                className="flex flex-wrap items-center gap-1.5"
                data-slot="task-fan-out-run-attribution"
              >
                <MonoId preserveCase size="sm" value={run.id} />
                {attribution ? (
                  <>
                    <span aria-hidden="true" className="text-faint">
                      →
                    </span>
                    <span>{attribution}</span>
                    {worktree?.branch ? (
                      <>
                        <span aria-hidden="true" className="text-faint">
                          ·
                        </span>
                        <MonoId preserveCase size="sm" value={worktree.branch} />
                      </>
                    ) : null}
                    {!failed && worktree ? <WorktreeOriginSignal origin={worktree.origin} /> : null}
                  </>
                ) : null}
              </ItemDescription>
              {run.error ? (
                <p
                  className="mt-px block text-badge leading-[1.45] text-danger"
                  data-slot="task-fan-out-run-error"
                >
                  {run.error}
                </p>
              ) : null}
            </ItemContent>
            {failed && onRetry ? (
              <ItemActions>
                <Button
                  data-testid="tasks-fan-out-run-retry"
                  disabled={retryPending}
                  onClick={() => onRetry(run.id)}
                  size="sm"
                  type="button"
                  variant="outline"
                >
                  Retry run
                </Button>
              </ItemActions>
            ) : null}
          </Item>
        );
      })}
    </div>
  );
}
