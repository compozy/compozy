import type * as React from "react";

import { Pill, Time } from "@agh/ui";

import { cn } from "@/lib/utils";

import { projectTaskExceptionPills } from "../lib/task-detail-pills";
import { taskOwnerLabel } from "../lib/task-formatters";
import type { TaskDetailView } from "../types";

function MetaDot() {
  return (
    <span aria-hidden="true" className="text-faint">
      ·
    </span>
  );
}

/**
 * Demoted meta line under the window head: exception pills first, then owner,
 * creator, and freshness. The head itself stays crumb + status + actions.
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
        "mb-4 flex min-w-0 flex-wrap items-center gap-2 border-b border-line pb-3.5 text-form-label text-subtle",
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
      <span>
        Owner <span className="font-medium text-muted">{taskOwnerLabel(record.owner)}</span>
      </span>
      <span className="inline-flex items-center gap-2">
        <MetaDot />
        <span>
          Created by{" "}
          <span className="font-medium text-muted">{record.created_by?.ref ?? "unknown"}</span>
        </span>
      </span>
      <span className="inline-flex items-center gap-2">
        <MetaDot />
        {closedAt ? (
          <span className="inline-flex items-center gap-1">
            Closed <Time iso={closedAt} mode="relative" />
          </span>
        ) : (
          <span className="inline-flex items-center gap-1">
            Updated <Time iso={record.updated_at} mode="relative" />
          </span>
        )}
      </span>
    </div>
  );
}
