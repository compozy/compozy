import { Link } from "@tanstack/react-router";
import { ListChecks } from "lucide-react";
import * as React from "react";

import { ListingRow, MonoId } from "@agh/ui";

import { formatRelativeTime, taskShortId } from "../lib/task-formatters";
import type { TaskListItem } from "../types";

export interface TasksListRowProps {
  task: TaskListItem;
  /** Optional slot rendered as the trail column. */
  trailing?: React.ReactNode;
  /**
   * Optional inline meta line rendered under the title row. Each child should
   * be a span; the row inserts `·` separators between adjacent children.
   */
  meta?: React.ReactNode;
  /** Test-id override. Defaults to the canonical row id `task-card-${task.id}`. */
  testId?: string;
  className?: string;
}

function MetaSeparator() {
  return (
    <span aria-hidden="true" className="text-faint opacity-60" data-slot="tasks-list-row-meta-sep">
      ·
    </span>
  );
}

function joinMeta(children: React.ReactNode): React.ReactNode[] {
  const items = React.Children.toArray(children);
  return items.flatMap((child, index) => {
    if (index === 0) return [child];
    const childKey = React.isValidElement(child) && child.key !== null ? child.key : String(child);
    return [<MetaSeparator key={`sep-${childKey}`} />, child];
  });
}

function TasksListRow({ task, trailing, meta, testId, className }: TasksListRowProps) {
  const identifier = taskShortId(task);
  const lastActivity = task.last_activity_at ?? task.updated_at;
  const timestamp = formatRelativeTime(lastActivity);
  const resolvedTestId = testId ?? `task-card-${task.id}`;

  return (
    <ListingRow
      className={className}
      data-slot="tasks-list-row"
      data-status={task.status}
      data-testid={resolvedTestId}
    >
      <ListingRow.Link
        render={<Link to="/tasks/$id" params={{ id: task.id }} aria-label={`Open ${task.title}`} />}
      >
        <ListingRow.Icon>
          <ListChecks aria-hidden="true" className="size-4" />
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title data-slot="tasks-list-row-title">{task.title}</ListingRow.Title>
          </ListingRow.Name>
          <ListingRow.Meta data-slot="tasks-list-row-meta">
            <MonoId value={identifier} size="sm" data-slot="tasks-list-row-id" />
            <MetaSeparator />
            <span
              className="font-mono text-badge tabular-nums text-faint"
              data-slot="tasks-list-row-timestamp"
            >
              {timestamp}
            </span>
            {meta !== undefined ? (
              <>
                <MetaSeparator />
                {joinMeta(meta)}
              </>
            ) : null}
          </ListingRow.Meta>
        </ListingRow.Main>
      </ListingRow.Link>
      {trailing !== undefined ? (
        <ListingRow.Trail data-slot="tasks-list-row-trailing">{trailing}</ListingRow.Trail>
      ) : null}
    </ListingRow>
  );
}

export { TasksListRow };
