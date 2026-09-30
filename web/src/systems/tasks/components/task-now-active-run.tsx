import { ArrowUpRight } from "lucide-react";

import { Button, StateGlyph, Time } from "@compozy/ui";
import type { TaskDetailView } from "../types";
import { TaskStateBand } from "./task-state-band";

type ActiveRun = NonNullable<NonNullable<TaskDetailView["summary"]>["active_run"]>;

interface TaskNowActiveRunProps {
  run: ActiveRun;
  maxAttempts?: number | null;
  onOpenRun: (runId: string) => void;
  elapsed?: string;
}

/** Active-run renderer for the task Overview Now strip. */
export function TaskNowActiveRun({ run, maxAttempts, onOpenRun, elapsed }: TaskNowActiveRunProps) {
  const isRunning = run.status === "running" || run.status === "starting";
  const attempts = maxAttempts
    ? `Attempt ${run.attempt} of ${maxAttempts}`
    : `Attempt ${run.attempt}`;
  const title = isRunning
    ? `${attempts} is running`
    : run.status === "claimed"
      ? `${attempts} is assigned`
      : `${attempts} is queued`;
  const claimant = run.claimed_by?.ref;

  return (
    <TaskStateBand
      actions={
        <>
          {elapsed ? (
            <span
              aria-label="Elapsed"
              className="text-form-label tabular-nums text-muted"
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
        </>
      }
      body={
        <>
          {claimant ? (
            <>
              <span className="font-medium text-fg">{claimant}</span> picked this up
            </>
          ) : (
            <>Waiting for an agent to pick this up</>
          )}
          {run.started_at ? (
            <>
              {" "}
              · started <Time iso={run.started_at} mode="relative" />
            </>
          ) : null}
        </>
      }
      data-slot="task-active-run-card"
      data-testid="tasks-detail-now-run"
      title={
        <span className="inline-flex items-center gap-2.5">
          <StateGlyph state="running" />
          {title}
        </span>
      }
      tone="accent"
    />
  );
}
