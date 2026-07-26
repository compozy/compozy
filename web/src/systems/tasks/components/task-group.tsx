import type { ReactNode } from "react";

import { Eyebrow, StatusDot } from "@agh/ui";
import { cn } from "@/lib/utils";

import { listGroupDotProps, type TaskListGroupId } from "../lib/task-grouping";

export interface TaskGroupProps {
  /** Canonical list-view group id (active / blocked / needs_attention / queued / done / failed). */
  id: TaskListGroupId;
  /** Group header label, rendered through the canonical `<Eyebrow>` utility. */
  label: string;
  /** Item count rendered next to the label as bare mono `--faint` text. */
  count: number;
  totalCount?: number;
  /** Row content for this group (typically `<TasksListRow>` / `<TaskCard>` siblings). */
  children: ReactNode;
  /** Optional right-aligned actions slot (e.g. inline `Add` affordance). */
  actions?: ReactNode;
  className?: string;
}

function TaskGroup({ id, label, count, totalCount, children, actions, className }: TaskGroupProps) {
  const { tone, variant } = listGroupDotProps(id);
  const countLabel =
    totalCount === undefined || totalCount === count ? `${count}` : `${count} of ${totalCount}`;

  return (
    <section
      aria-label={label}
      data-slot="task-group"
      data-group-id={id}
      data-testid={`task-group-${id}`}
      className={cn("flex min-w-0 flex-col gap-2", className)}
    >
      <header className="flex items-center gap-2 px-1" data-slot="task-group-head">
        <StatusDot
          data-testid={`task-group-dot-${id}`}
          label={label}
          tone={tone}
          variant={variant}
        />
        <Eyebrow className="text-muted" data-testid={`task-group-${id}-label`}>
          {label}
        </Eyebrow>
        <span
          aria-hidden="true"
          className="font-mono text-mono-id tabular-nums text-faint"
          data-slot="task-group-count"
          data-testid={`task-group-${id}-count`}
        >
          {countLabel}
        </span>
        {actions ? (
          <div className="ml-auto flex items-center gap-1" data-slot="task-group-actions">
            {actions}
          </div>
        ) : null}
      </header>
      <div
        className="overflow-hidden rounded-lg border border-line bg-canvas-soft"
        data-slot="task-group-rows"
      >
        {children}
      </div>
    </section>
  );
}

export { TaskGroup };
