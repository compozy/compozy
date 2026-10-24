import * as React from "react";

import {
  Skeleton,
  SkeletonText,
  StateGlyph,
  type StateGlyphState,
  TableCell,
  TableRow,
} from "@compozy/ui";
import { cn } from "@/lib/utils";

import { TASKS_TABLE_COLUMN_CLASS } from "../lib/tasks-table-columns";

/** Cell classes per column, shared by the row and its skeleton so their columns cannot drift. */
const CELL_CLASS = {
  title: "max-w-0 pl-4",
  id: cn("max-w-0", TASKS_TABLE_COLUMN_CLASS.id),
  status: TASKS_TABLE_COLUMN_CLASS.status,
  priority: cn("text-muted", TASKS_TABLE_COLUMN_CLASS.priority),
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
              className="flex min-w-0 shrink-[4] items-center gap-1.5 truncate text-meta text-muted"
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

export interface TasksListRowSkeletonProps {
  /** Width utility for the title placeholder, e.g. `w-56`. */
  titleWidth: string;
  /** Width utility for the quiet meta placeholder after the title. */
  metaWidth: string;
}

/**
 * The placeholder twin of `TasksListRow`: the same row and the same column cells, with each
 * content slot a block of text, so the data arrives into rows of the same height and columns.
 * Place inside a `TaskGroupSkeleton` and wrap the whole table in a `SkeletonRegion`.
 */
function TasksListRowSkeleton({ titleWidth, metaWidth }: TasksListRowSkeletonProps) {
  return (
    <TableRow aria-hidden="true" className="animate-skeleton-reveal hover:bg-transparent">
      <TableCell className={CELL_CLASS.title}>
        <div className="flex min-w-0 items-center gap-2">
          <SkeletonText className={titleWidth} />
          <SkeletonText className={cn("text-meta", metaWidth)} />
        </div>
      </TableCell>
      <TableCell className={CELL_CLASS.id}>
        <SkeletonText className="w-16" />
      </TableCell>
      <TableCell className={CELL_CLASS.status}>
        <span className="inline-flex items-center gap-1.75">
          <Skeleton className="size-3.5 shrink-0 rounded-full" />
          <SkeletonText className="w-16" />
        </span>
      </TableCell>
      <TableCell className={CELL_CLASS.priority}>
        <SkeletonText className="w-12" />
      </TableCell>
      <TableCell className={CELL_CLASS.owner}>
        <Skeleton className="size-avatar-sm rounded-full" />
      </TableCell>
      <TableCell className={CELL_CLASS.updated}>
        <SkeletonText className="w-10" />
      </TableCell>
    </TableRow>
  );
}

export { TasksListRow, TasksListRowSkeleton };
