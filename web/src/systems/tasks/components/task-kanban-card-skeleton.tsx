import { Skeleton, SkeletonText } from "@compozy/ui";

import { cn } from "@/lib/utils";

import { CARD_SURFACE_CLASS } from "./task-kanban-card-classes";

export interface TaskKanbanCardSkeletonProps {
  /** Title lines the placeholder stands in for: the real title clamps at two. */
  titleLines?: 1 | 2;
}

/**
 * The placeholder twin of `TaskKanbanCard`: the same card surface and rows (title, identifier,
 * owner and recency) with each content slot a block of text, so the data arrives into a card of
 * the same box. Place inside a `TaskKanbanColumn` and wrap the board in a `SkeletonRegion`.
 */
export function TaskKanbanCardSkeleton({ titleLines = 1 }: TaskKanbanCardSkeletonProps) {
  return (
    <div
      aria-hidden="true"
      className={cn(CARD_SURFACE_CLASS, "animate-skeleton-reveal")}
      data-testid="tasks-kanban-card-skeleton"
    >
      <div className="min-w-0 text-small-body leading-snug">
        <SkeletonText lastLineWidth={0.55} lines={titleLines} />
      </div>
      <div className="flex min-w-0 items-center gap-2">
        <SkeletonText className="w-14 font-mono text-mono-id leading-3" />
      </div>
      <div className="flex min-w-0 items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-1.5">
          <Skeleton className="size-avatar-sm shrink-0 rounded-full" />
          <SkeletonText className="w-20 text-eyebrow" />
        </div>
        <SkeletonText className="w-8 font-mono text-badge" />
      </div>
    </div>
  );
}
