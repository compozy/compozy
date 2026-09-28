import type * as React from "react";

import { Pill, Time } from "@compozy/ui";

import { cn } from "@/lib/utils";

import { projectTaskExceptionPills } from "../lib/task-detail-pills";
import type { TaskDetailView } from "../types";

/**
 * Demoted meta line under the window head: exception pills, then freshness.
 * Owner and creator live in the properties rail.
 */
export interface TasksDetailSubheadProps extends Omit<React.ComponentProps<"div">, "children"> {
  detail: TaskDetailView;
}

export function TasksDetailSubhead({ detail, className, ...props }: TasksDetailSubheadProps) {
  const record = detail.task;
  const pills = projectTaskExceptionPills(detail);
  const closedAt = record.closed_at ?? null;

  return (
    <div
      className={cn(
        "mb-5 flex min-w-0 flex-wrap items-center gap-2 border-b border-line pb-4 text-form-label text-subtle",
        className
      )}
      data-testid="tasks-detail-subhead"
      {...props}
    >
      {pills.map(pill => (
        <Pill
          data-testid={`tasks-detail-pill-${pill.key}`}
          key={pill.key}
          title={pill.title}
          tone={pill.tone}
        >
          {pill.label}
        </Pill>
      ))}
      {closedAt ? (
        <span className="inline-flex items-center gap-1">
          Closed <Time iso={closedAt} mode="relative" />
        </span>
      ) : (
        <span className="inline-flex items-center gap-1">
          Updated <Time iso={record.updated_at} mode="relative" />
        </span>
      )}
    </div>
  );
}
