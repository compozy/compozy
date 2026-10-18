import type { ReactNode } from "react";

import { Eyebrow, StateGlyph, TableBody } from "@compozy/ui";
import { cn } from "@/lib/utils";

import { listGroupGlyph, type TaskListGroupId } from "../lib/task-grouping";
import { TASKS_TABLE_COLUMN_CLASS } from "../lib/tasks-table-columns";

const HEAD_FILLER_COLUMNS = ["id", "status", "priority", "owner", "updated"] as const;

export interface TaskGroupProps {
  /** Canonical list-view group id (active / blocked / needs_attention / queued / done / failed). */
  id: TaskListGroupId;
  /** Group header label, rendered through the canonical `<Eyebrow>` utility. */
  label: string;
  /** Loaded row count rendered next to the label. */
  count: number;
  totalCount?: number;
  /** Table rows for this group (`<TaskCard>` siblings). */
  children: ReactNode;
  className?: string;
}

/**
 * One status bucket of the Tasks table: a `tbody` led by a sunken header row
 * carrying the bucket's state glyph, label and count.
 */
function TaskGroup({ id, label, count, totalCount, children, className }: TaskGroupProps) {
  const countLabel =
    totalCount === undefined || totalCount === count ? `${count}` : `${count} of ${totalCount}`;

  return (
    <TableBody
      aria-label={label}
      className={cn("[&_tr:last-child]:border-b", className)}
      data-group-id={id}
      data-slot="task-group"
      data-testid={`task-group-${id}`}
    >
      <tr className="border-b border-line-soft bg-sunken" data-slot="task-group-head">
        <th className="h-9 px-4 text-left font-normal whitespace-nowrap" scope="rowgroup">
          <span className="inline-flex items-center gap-2">
            <StateGlyph data-testid={`task-group-dot-${id}`} state={listGroupGlyph(id)} />
            <Eyebrow className="text-fg-2" data-testid={`task-group-${id}-label`}>
              {label}
            </Eyebrow>
            <span
              aria-hidden="true"
              className="text-eyebrow tabular-nums text-subtle"
              data-slot="task-group-count"
              data-testid={`task-group-${id}-count`}
            >
              {countLabel}
            </span>
          </span>
        </th>
        {/* One cell per column rather than a colSpan: a span would keep the
            columns the pane width hides alive as empty tracks. */}
        {HEAD_FILLER_COLUMNS.map(column => (
          <td className={TASKS_TABLE_COLUMN_CLASS[column]} key={column} />
        ))}
      </tr>
      {children}
    </TableBody>
  );
}

export { TaskGroup };
