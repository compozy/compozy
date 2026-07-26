import { cn, Eyebrow, Panel, Pill, type PillTone } from "@agh/ui";

import { formatPercent, taskStatusLabel, taskStatusTone } from "../lib/task-formatters";
import type { TaskDashboardView } from "../types";

export interface TasksDashboardStatusBreakdownProps {
  dashboard: TaskDashboardView;
}

const TONE_FILL_CLASS: Record<PillTone, string> = {
  neutral: "bg-neutral",
  accent: "bg-accent",
  success: "bg-success",
  warning: "bg-warning",
  danger: "bg-danger",
  info: "bg-info",
};

export function TasksDashboardStatusBreakdown({ dashboard }: TasksDashboardStatusBreakdownProps) {
  const entries = dashboard.status_breakdown ?? [];
  const total = entries.reduce((sum, entry) => sum + entry.count, 0);

  return (
    <Panel
      data-testid="tasks-dashboard-status-breakdown"
      right={
        total > 0 ? (
          <Eyebrow className="text-muted" data-testid="tasks-dashboard-status-breakdown-total">
            total {total}
          </Eyebrow>
        ) : undefined
      }
      title="Status breakdown"
    >
      {entries.length === 0 ? (
        <p
          className="text-form-label text-muted"
          data-testid="tasks-dashboard-status-breakdown-empty"
        >
          No task activity yet.
        </p>
      ) : (
        <ul className="flex flex-col gap-3">
          {entries.map(entry => {
            const tone = taskStatusTone(entry.status);
            const sharePct = Math.max(0, Math.min(100, entry.share_percent ?? 0));
            return (
              <li
                className="flex flex-col gap-1.5"
                data-testid={`tasks-dashboard-status-row-${entry.status}`}
                key={entry.status}
              >
                <div className="flex min-w-0 items-center gap-2 text-form-label">
                  <Pill.Dot tone={tone} size="sm" />
                  <span
                    className="min-w-0 flex-1 truncate font-medium text-fg"
                    data-testid={`tasks-dashboard-status-label-${entry.status}`}
                  >
                    {taskStatusLabel(entry.status)}
                  </span>
                  <span
                    className="shrink-0 font-mono text-mono-id tabular-nums text-faint"
                    data-testid={`tasks-dashboard-status-count-${entry.status}`}
                  >
                    {entry.count}
                  </span>
                  <span
                    className="w-12 shrink-0 text-right font-mono text-eyebrow font-medium tabular-nums text-muted"
                    data-testid={`tasks-dashboard-status-share-${entry.status}`}
                  >
                    {formatPercent(sharePct)}
                  </span>
                </div>
                <div
                  aria-hidden="true"
                  className="ml-4 h-1 overflow-hidden rounded-xs bg-surface-glaze"
                >
                  <div
                    className={cn("h-full rounded-xs", TONE_FILL_CLASS[tone])}
                    data-testid={`tasks-dashboard-status-bar-${entry.status}`}
                    style={{ width: `${sharePct}%` }}
                  />
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </Panel>
  );
}
