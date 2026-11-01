import { ChartBar } from "@compozy/ui/icons";

import {
  cn,
  Empty,
  Eyebrow,
  Panel,
  Pill,
  type PillTone,
  Skeleton,
  SkeletonText,
} from "@compozy/ui";

import { formatPercent, taskStatusLabel, taskStatusTone } from "../lib/task-formatters";
import type { TaskDashboardView } from "../types";

export interface TasksDashboardStatusBreakdownProps {
  dashboard: TaskDashboardView;
}

const TONE_FILL_CLASS: Record<PillTone, string> = {
  neutral: "bg-neutral",
  brand: "bg-brand",
  success: "bg-success",
  warning: "bg-warning",
  destructive: "bg-destructive",
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
          <Eyebrow
            className="text-muted-foreground"
            data-testid="tasks-dashboard-status-breakdown-total"
          >
            total {total}
          </Eyebrow>
        ) : undefined
      }
      title="Status breakdown"
    >
      {entries.length === 0 ? (
        <Empty
          data-testid="tasks-dashboard-status-breakdown-empty"
          icon={ChartBar}
          size="compact"
          title="No task activity yet."
        />
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
                    className="w-12 shrink-0 text-right font-mono text-eyebrow font-medium tabular-nums text-muted-foreground"
                    data-testid={`tasks-dashboard-status-share-${entry.status}`}
                  >
                    {formatPercent(sharePct)}
                  </span>
                </div>
                <div
                  aria-hidden="true"
                  className="ml-4 h-1 overflow-hidden rounded-xs bg-surface-2"
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

/** Rows a loading panel stands in with; ids are static, the placeholders carry no identity. */
const LOADING_STATUSES = [
  { id: "a" },
  { id: "b" },
  { id: "c" },
  { id: "d" },
  { id: "e" },
  { id: "f" },
] as const;

/**
 * The placeholder twin of the status breakdown: the same panel head and rows (a state dot, a
 * label, a count and share, and the share bar), with the figures left as blocks. Wrap the
 * dashboard area in a `SkeletonRegion`.
 */
export function TasksDashboardStatusBreakdownSkeleton() {
  return (
    <Panel aria-hidden="true" className="animate-skeleton-reveal" title="Status breakdown">
      <ul className="flex flex-col gap-3">
        {LOADING_STATUSES.map(status => (
          <li className="flex flex-col gap-1.5" key={status.id}>
            <div className="flex min-w-0 items-center gap-2 text-form-label">
              <Skeleton className="size-1.5 shrink-0 rounded-full" />
              <SkeletonText className="w-24 flex-1" />
              <SkeletonText className="w-6 shrink-0 font-mono text-mono-id" />
              <SkeletonText className="w-12 shrink-0 font-mono text-eyebrow" />
            </div>
            <Skeleton className="ml-4 h-1 rounded-xs" />
          </li>
        ))}
      </ul>
    </Panel>
  );
}
