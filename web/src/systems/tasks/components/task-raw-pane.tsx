import type * as React from "react";

import { cn, JsonViewer } from "@agh/ui";

import type { TaskDetailView } from "../types";

/**
 * Inspect drawer › Raw pane: the task record and (when present) the active
 * run DTO, verbatim. The operator floor — nothing rendered above hides this.
 */
export interface TaskRawPaneProps extends Omit<React.ComponentProps<"div">, "children"> {
  detail: TaskDetailView;
}

export function TaskRawPane({ detail, className, ...props }: TaskRawPaneProps) {
  const activeRun = detail.summary?.active_run ?? null;
  return (
    <div
      className={cn("flex flex-col gap-4", className)}
      data-testid="tasks-inspect-raw"
      {...props}
    >
      <section className="flex flex-col gap-2">
        <span className="eyebrow text-subtle">Task</span>
        <JsonViewer value={detail.task} />
      </section>
      {activeRun ? (
        <section className="flex flex-col gap-2">
          <span className="eyebrow text-subtle">Active run</span>
          <JsonViewer value={activeRun} />
        </section>
      ) : null}
    </div>
  );
}
