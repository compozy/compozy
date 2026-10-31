import { Link } from "@tanstack/react-router";
import { Activity, AlertCircle, ChevronRight } from "@compozy/ui/icons";

import { Empty, Eyebrow, Panel, Pill, StateGlyph } from "@compozy/ui";

import { cn } from "@/lib/utils";
import { WorkspaceBadge, type WorkspaceBadgeIdentity } from "@/systems/workspace";

import {
  formatAttemptLabel,
  formatDurationMs,
  taskRunStateGlyph,
  taskRunStatusLabel,
} from "../lib/task-formatters";
import { taskWindowBinding } from "../lib/task-detail-search";
import type { TaskDashboardView } from "../types";
import { RUN_LINE_CLASS, RUN_ROW_CLASS } from "./tasks-dashboard-active-runs-classes";

export interface TasksDashboardActiveRunsProps {
  dashboard: TaskDashboardView;
  maxItems?: number;
  /** Each run's project under All projects; the run then opens bound to it. */
  projectBadgeOf?: (run: { workspace_id?: string }) => WorkspaceBadgeIdentity;
}

export function TasksDashboardActiveRuns({
  dashboard,
  maxItems = 6,
  projectBadgeOf,
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
        <Eyebrow className="text-muted-foreground">
          {running} running · {queued} queued · {claimed} claimed
        </Eyebrow>
      }
      title="Active runs"
    >
      {visible.length === 0 ? (
        <Empty
          className="px-5 py-6"
          data-testid="tasks-dashboard-active-runs-empty"
          icon={Activity}
          size="compact"
          title="No active runs right now."
        />
      ) : (
        <ul className="divide-y divide-line-soft" data-testid="tasks-dashboard-active-runs-list">
          {visible.map(run => {
            const attemptLabel = formatAttemptLabel(run.attempt, run.max_attempts);
            const projectBadge = projectBadgeOf?.(run);
            return (
              <li
                className={cn(RUN_ROW_CLASS, "transition-colors hover:bg-surface-2")}
                data-testid={`tasks-dashboard-active-run-${run.run_id}`}
                key={run.run_id}
              >
                <Link
                  className={cn(
                    "group outline-none",
                    RUN_LINE_CLASS,
                    "focus-visible:rounded-xs focus-visible:shadow-focus-ring"
                  )}
                  data-testid={`tasks-dashboard-active-run-link-${run.run_id}`}
                  params={{ id: run.task_id, runId: run.run_id }}
                  search={projectBadge ? taskWindowBinding(run.workspace_id) : undefined}
                  to="/tasks/$id/runs/$runId"
                >
                  <StateGlyph state={taskRunStateGlyph(run.run_status)} />
                  <div className="flex min-w-0 flex-1 items-baseline gap-2">
                    <span className="min-w-0 truncate text-small-body font-medium text-fg-strong">
                      {run.task_title}
                    </span>
                    {run.task_identifier ? (
                      <span className="shrink-0 font-mono text-mono-id tabular-nums text-faint">
                        {run.task_identifier}
                      </span>
                    ) : null}
                    {projectBadge ? (
                      <WorkspaceBadge
                        className="max-w-40 shrink-[1000]"
                        data-testid={`tasks-dashboard-active-run-project-${run.run_id}`}
                        identity={projectBadge}
                      />
                    ) : null}
                  </div>
                  <span className="hidden shrink-0 text-form-label tabular-nums text-muted-foreground @xl:inline">
                    for {formatDurationMs(run.age_ms)}
                  </span>
                  {attemptLabel ? (
                    <span className="hidden shrink-0 text-form-label tabular-nums text-muted-foreground @3xl:inline">
                      {attemptLabel}
                    </span>
                  ) : null}
                  <span className="shrink-0 text-form-label text-fg-2">
                    {taskRunStatusLabel(run.run_status)}
                  </span>
                  {run.stuck ? (
                    <Pill
                      data-testid={`tasks-dashboard-active-run-stuck-${run.run_id}`}
                      size="sm"
                      tone="destructive"
                    >
                      Stuck
                    </Pill>
                  ) : null}
                  <ChevronRight
                    aria-hidden="true"
                    className="size-3.5 shrink-0 text-faint transition-colors group-hover:text-muted-foreground"
                  />
                </Link>
                {run.error ? (
                  <p
                    className="flex items-start gap-1.5 pl-6 text-form-hint text-destructive"
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
          className="border-t border-line-soft px-5 py-3 text-form-label text-muted-foreground"
          data-testid="tasks-dashboard-active-runs-more"
        >
          +{hidden} more active runs
        </p>
      ) : null}
    </Panel>
  );
}
