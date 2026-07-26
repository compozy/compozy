import { ArrowUpRight } from "lucide-react";

import { Button, Time } from "@agh/ui";

import { useLiveElapsed } from "../hooks/use-live-elapsed";
import { runCoordinationChannelLabel, runIsCoordinated } from "../lib/task-formatters";
import type { TaskDetailView } from "../types";

type ActiveRun = NonNullable<NonNullable<TaskDetailView["summary"]>["active_run"]>;

interface TaskNowActiveRunProps {
  run: ActiveRun;
  maxAttempts?: number | null;
  onOpenRun: (runId: string) => void;
}

/** Active-run renderer for the task Overview Now strip. */
export function TaskNowActiveRun({ run, maxAttempts, onOpenRun }: TaskNowActiveRunProps) {
  const isRunning = run.status === "running" || run.status === "starting";
  const elapsed = useLiveElapsed(run.started_at ?? undefined, isRunning);
  const attempts = maxAttempts
    ? `Attempt ${run.attempt} of ${maxAttempts}`
    : `Attempt ${run.attempt}`;
  const title = isRunning
    ? `${attempts} is running`
    : run.status === "claimed"
      ? `${attempts} is assigned`
      : `${attempts} is queued`;
  const claimant = run.claimed_by?.ref;
  const coordinationChannel = runIsCoordinated(run) ? runCoordinationChannelLabel(run) : null;

  return (
    <section
      className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-4 rounded-lg border border-accent-dim bg-canvas-soft px-4 py-3.5"
      data-slot="task-active-run-card"
      data-testid="tasks-detail-now-run"
    >
      <div className="min-w-0">
        <div className="flex items-center gap-2.5 text-card-title font-medium text-fg-strong">
          <span
            aria-hidden="true"
            className="size-[7px] shrink-0 rounded-full bg-accent motion-safe:animate-pulse"
          />
          {title}
        </div>
        <p className="mt-1 text-small-body text-muted">
          {claimant ? (
            <>
              <span className="font-medium text-fg">{claimant}</span> picked this up
            </>
          ) : (
            <>Waiting for a worker to pick this up</>
          )}
          {run.started_at ? (
            <>
              {" "}
              · started <Time iso={run.started_at} mode="relative" />
            </>
          ) : null}
        </p>
        {coordinationChannel ? (
          <p
            className="mt-1 text-form-label text-subtle"
            data-testid="tasks-detail-coordination"
            title="Channel messages support coordination only; task-run status remains authoritative."
          >
            Coordination channel · {coordinationChannel}
          </p>
        ) : null}
      </div>
      <div className="flex shrink-0 items-center gap-3.5">
        {elapsed ? (
          <span
            aria-label="Elapsed"
            className="font-mono text-form-label tabular-nums text-muted"
            data-testid="tasks-detail-now-elapsed"
          >
            {elapsed}
          </span>
        ) : null}
        <Button
          className="min-h-6"
          data-testid="tasks-detail-now-open-run"
          onClick={() => onOpenRun(run.id)}
          size="sm"
          type="button"
          variant="ghost"
        >
          Open run
          <ArrowUpRight aria-hidden="true" className="size-3" />
        </Button>
      </div>
    </section>
  );
}
