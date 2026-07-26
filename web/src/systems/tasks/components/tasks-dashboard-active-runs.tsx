import { Link } from "@tanstack/react-router";
import { AlertCircle, ChevronRight } from "lucide-react";

import { Eyebrow, Pill, Panel } from "@agh/ui";

import { cn } from "@/lib/utils";

import {
  formatAttemptLabel,
  formatDurationMs,
  taskRunStatusTone,
  taskStatusSignal,
} from "../lib/task-formatters";
import type { TaskDashboardView } from "../types";

export interface TasksDashboardActiveRunsProps {
  dashboard: TaskDashboardView;
  maxItems?: number;
}

export function TasksDashboardActiveRuns({
  dashboard,
  maxItems = 6,
}: TasksDashboardActiveRunsProps) {
  const runs = dashboard.active_runs.items ?? [];
  const visible = runs.slice(0, maxItems);
  const hidden = runs.length - visible.length;
  const { running, queued, claimed, total } = dashboard.active_runs;

  return (
    <Panel
      bodyClassName="p-0"
      data-testid="tasks-dashboard-active-runs"
      meta={total}
      right={
        <Eyebrow className="text-muted">
          {running} running · {queued} queued · {claimed} claimed
        </Eyebrow>
      }
      title="Active runs"
    >
      {visible.length === 0 ? (
        <p
          className="px-5 py-6 text-form-label text-muted"
          data-testid="tasks-dashboard-active-runs-empty"
        >
          No active runs right now.
        </p>
      ) : (
        <ul className="divide-y divide-line-soft" data-testid="tasks-dashboard-active-runs-list">
          {visible.map(run => {
            const signal = taskStatusSignal(run.task_status);
            const attemptLabel = formatAttemptLabel(run.attempt, run.max_attempts);
            return (
              <li
                className="flex flex-col gap-1.5 px-5 py-2.5 transition-colors hover:bg-row-hover"
                data-testid={`tasks-dashboard-active-run-${run.run_id}`}
                key={run.run_id}
              >
                <Link
                  className={cn(
                    "group flex min-w-0 items-center gap-3 text-form-label outline-none",
                    "focus-visible:rounded-xs focus-visible:shadow-focus-ring"
                  )}
                  data-testid={`tasks-dashboard-active-run-link-${run.run_id}`}
                  params={{ id: run.task_id, runId: run.run_id }}
                  to="/tasks/$id/runs/$runId"
                >
                  <Pill.Dot tone={signal.tone} pulse={signal.pulse} size="sm" />
                  <div className="flex min-w-0 flex-1 items-baseline gap-2">
                    <span className="min-w-0 truncate text-section-head font-medium tracking-section-head text-fg-strong">
                      {run.task_title}
                    </span>
                    {run.task_identifier ? (
                      <span className="shrink-0 font-mono text-mono-id tabular-nums text-faint">
                        {run.task_identifier}
                      </span>
                    ) : null}
                  </div>
                  <span className="hidden shrink-0 font-mono text-mono-id tabular-nums text-muted md:inline">
                    age {formatDurationMs(run.age_ms)}
                  </span>
                  {attemptLabel ? (
                    <span className="hidden shrink-0 font-mono text-mono-id tabular-nums text-muted lg:inline">
                      {attemptLabel}
                    </span>
                  ) : null}
                  <Pill size="sm" tone={taskRunStatusTone(run.run_status)}>
                    {run.run_status}
                  </Pill>
                  {run.stuck ? (
                    <Pill
                      data-testid={`tasks-dashboard-active-run-stuck-${run.run_id}`}
                      size="sm"
                      tone="danger"
                    >
                      stuck
                    </Pill>
                  ) : null}
                  <ChevronRight
                    aria-hidden="true"
                    className="size-3 shrink-0 text-faint transition-colors group-hover:text-muted"
                  />
                </Link>
                {run.error ? (
                  <p
                    className="flex items-start gap-1.5 pl-3.5 text-form-hint text-danger"
                    data-testid={`tasks-dashboard-active-run-error-${run.run_id}`}
                  >
                    <AlertCircle aria-hidden="true" className="mt-0.5 size-3 shrink-0" />
                    <span className="min-w-0 truncate">{run.error}</span>
                  </p>
                ) : null}
              </li>
            );
          })}
        </ul>
      )}

      {hidden > 0 ? (
        <p
          className="border-t border-line-soft px-5 py-3 text-form-label text-muted"
          data-testid="tasks-dashboard-active-runs-more"
        >
          +{hidden} more active runs
        </p>
      ) : null}
    </Panel>
  );
}
