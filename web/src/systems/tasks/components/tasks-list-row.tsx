import * as React from "react";

import {
  Skeleton,
  SkeletonText,
  StateGlyph,
  type StateGlyphState,
  TableCell,
  TableRow,
  type TableRowSkeletonColumn,
} from "@compozy/ui";
import { cn } from "@/lib/utils";

import { TASKS_TABLE_COLUMN_CLASS } from "../lib/tasks-table-columns";

/** Cell classes per column, shared by the row and its skeleton so their columns cannot drift. */
const CELL_CLASS = {
  title: "max-w-0 pl-4",
  id: cn("max-w-0", TASKS_TABLE_COLUMN_CLASS.id),
  status: TASKS_TABLE_COLUMN_CLASS.status,
  priority: cn("text-muted-foreground", TASKS_TABLE_COLUMN_CLASS.priority),
  owner: cn("relative z-1 pr-4 @5xl:pr-3", TASKS_TABLE_COLUMN_CLASS.owner),
  updated: cn(
    "pr-4 font-mono text-meta tabular-nums text-subtle",
    TASKS_TABLE_COLUMN_CLASS.updated
  ),
} as const;

export interface TasksListRowProps extends Omit<React.ComponentProps<"tr">, "title" | "id"> {
  /**
   * Router link rendered over the title. Its hit area stretches across the row,
   * so the whole row opens the record while trailing controls stay outside it.
   * `null` renders an inert row (e.g. a Loop record whose run is gone).
   */
  link: React.ReactElement<{ className?: string; children?: React.ReactNode }> | null;
  title: React.ReactNode;
  /** Quiet inline facts after the title. Each child should be a span; `·` joins them. */
  meta?: React.ReactNode;
  id?: React.ReactNode;
  state: StateGlyphState;
  statusLabel: React.ReactNode;
  priority?: React.ReactNode;
  owner?: React.ReactNode;
  updated?: React.ReactNode;
}

function MetaSeparator() {
  return (
    <span aria-hidden="true" className="text-subtle" data-slot="tasks-list-row-meta-sep">
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

/** One Tasks table row: title and quiet meta, identifier, state glyph, priority, owner, recency. */
function TasksListRow({
  link,
  title,
  meta,
  id,
  state,
  statusLabel,
  priority,
  owner,
  updated,
  className,
  ...props
}: TasksListRowProps) {
  const metaItems = meta === undefined ? [] : joinMeta(meta);
  const titleText = (
    <span className="min-w-0 truncate font-medium text-fg" data-slot="tasks-list-row-title">
      {title}
    </span>
  );
  return (
    <TableRow
      className={cn("relative", link === null && "hover:bg-transparent", className)}
      data-slot="tasks-list-row"
      {...props}
    >
      <TableCell className={CELL_CLASS.title}>
        <div className="flex min-w-0 items-center gap-2">
          {link
            ? React.cloneElement(
                link,
                {
                  className:
                    "flex min-w-0 shrink items-center outline-none after:absolute after:inset-0 after:content-[''] focus-visible:after:shadow-focus-ring",
                },
                titleText
              )
            : titleText}
          {metaItems.length > 0 ? (
            <span
              className="flex min-w-0 shrink-[4] items-center gap-1.5 truncate text-meta text-muted-foreground"
              data-slot="tasks-list-row-meta"
            >
              {metaItems}
            </span>
          ) : null}
        </div>
      </TableCell>
      <TableCell className={CELL_CLASS.id}>
        {id ? (
          <span
            className="block truncate font-mono text-meta tabular-nums text-fg-2"
            data-slot="tasks-list-row-id"
          >
            {id}
          </span>
        ) : null}
      </TableCell>
      <TableCell className={CELL_CLASS.status}>
        <span
          className="inline-flex items-center gap-1.75 text-fg-2"
          data-slot="tasks-list-row-status"
        >
          <StateGlyph state={state} />
          {statusLabel}
        </span>
      </TableCell>
      <TableCell className={CELL_CLASS.priority}>{priority}</TableCell>
      <TableCell className={CELL_CLASS.owner}>{owner}</TableCell>
      <TableCell className={CELL_CLASS.updated} data-slot="tasks-list-row-updated">
        {updated}
      </TableCell>
    </TableRow>
  );
}

const TITLE_WIDTHS = ["w-56", "w-40", "w-64", "w-48", "w-44", "w-60"] as const;
const META_WIDTHS = ["w-24", "w-16", "w-28", "w-20", "w-20", "w-24"] as const;

/**
 * The placeholder twin of `TasksListRow`, as the columns `TableRowSkeleton` draws: each column
 * carries the real row's own cell classes, including the display class that hides it in a narrow
 * pane, and a placeholder in the shape of what the cell holds, so the data arrives into rows of
 * the same height and columns. Fill a `TaskGroupSkeleton` with it and wrap the table in a
 * `SkeletonRegion`.
 */
const TASKS_LIST_SKELETON_COLUMNS: readonly TableRowSkeletonColumn[] = [
  {
    cellClassName: CELL_CLASS.title,
    placeholder: row => (
      <div className="flex min-w-0 items-center gap-2">
        <SkeletonText className={TITLE_WIDTHS[row % TITLE_WIDTHS.length]} />
        <SkeletonText className={cn("text-meta", META_WIDTHS[row % META_WIDTHS.length])} />
      </div>
    ),
  },
  { cellClassName: CELL_CLASS.id, className: "w-16" },
  {
    cellClassName: CELL_CLASS.status,
    placeholder: () => (
      <span className="inline-flex items-center gap-1.75">
        <Skeleton className="size-3.5 shrink-0 rounded-full" />
        <SkeletonText className="w-16" />
      </span>
    ),
  },
  { cellClassName: CELL_CLASS.priority, className: "w-12" },
  {
    cellClassName: CELL_CLASS.owner,
    placeholder: () => <Skeleton className="size-avatar-sm rounded-full" />,
  },
  { cellClassName: CELL_CLASS.updated, className: "w-10" },
];

export { TASKS_LIST_SKELETON_COLUMNS, TasksListRow };
