import * as React from "react";

import { cn } from "@/lib/utils";

import type { InboxGroupId } from "../lib/inbox-grouping";

export interface TasksInboxRowProps extends Omit<React.ComponentProps<"div">, "onSelect"> {
  taskId: string;
  /**
   * Inbox group this row belongs to. Drives the rail tone —
   * `needs_review` paints warning, `blocked` paints danger, and `updates`
   * paints a faint ring.
   */
  group: InboxGroupId;
  unread?: boolean;
  onSelect?: () => void;
  /** Top row content -- title + identifier + status/lane badges. */
  top: React.ReactNode;
  /** Optional detail rows under the top (blocking reason, error, owner · time). */
  detail?: React.ReactNode;
  /** Right-side meta column (auto-width) — owner avatar, timestamp, etc. */
  meta?: React.ReactNode;
  /** Right-side actions (auto-width column). */
  actions?: React.ReactNode;
}

/**
 * Inbox row primitive — 3-column grid `[ rail | body | meta ]`. The rail
 * carries the group tone from backend-backed signals. Unread state is
 * expressed via the body's title weight.
 */
const RAIL_CLASS: Record<InboxGroupId, string> = {
  needs_review: "bg-warning",
  blocked: "bg-danger",
  updates: "bg-transparent shadow-inset-strong",
};

function TasksInboxRow({
  taskId,
  group,
  unread = false,
  onSelect,
  top,
  detail,
  meta,
  actions,
  className,
  ...props
}: TasksInboxRowProps) {
  const clickable = onSelect !== undefined;
  const handleKeyDown = clickable
    ? (event: React.KeyboardEvent<HTMLDivElement>) => {
        if (event.target !== event.currentTarget) return;
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onSelect?.();
        }
      }
    : undefined;

  const trailing = meta !== undefined || actions !== undefined;

  return (
    <div
      data-slot="tasks-inbox-row"
      data-group={group}
      data-testid={`tasks-inbox-item-${taskId}`}
      data-unread={unread ? "true" : "false"}
      onClick={clickable ? () => onSelect?.() : undefined}
      onKeyDown={handleKeyDown}
      role={clickable ? "button" : undefined}
      tabIndex={clickable ? 0 : undefined}
      className={cn(
        "grid min-h-11 items-stretch gap-3 border-b border-line-soft py-2.5 pr-3.5 text-left transition-colors duration-base ease-out",
        trailing ? "grid-cols-[3px_minmax(0,1fr)_auto]" : "grid-cols-[3px_minmax(0,1fr)]",
        clickable &&
          "cursor-pointer hover:bg-row-hover focus-visible:outline-none focus-visible:shadow-focus-inset",
        className
      )}
      {...props}
    >
      <span
        aria-hidden="true"
        data-slot="tasks-inbox-row-rail"
        className={cn("self-stretch rounded-r-xs", RAIL_CLASS[group])}
      />

      <div className="flex min-w-0 flex-col gap-1 pl-2" data-slot="tasks-inbox-row-main">
        <div className="flex min-w-0 flex-wrap items-center gap-2" data-slot="tasks-inbox-row-top">
          {top}
        </div>
        {detail !== undefined ? (
          <div
            className="flex min-w-0 flex-col gap-1 text-small-body text-muted"
            data-slot="tasks-inbox-row-detail"
          >
            {detail}
          </div>
        ) : null}
      </div>

      {trailing ? (
        <div
          className="flex shrink-0 items-center gap-1.5"
          data-slot="tasks-inbox-row-meta"
          data-testid={`tasks-inbox-item-actions-${taskId}`}
          onClick={stopPropagation}
          onKeyDown={stopPropagation}
          role="presentation"
        >
          {meta}
          {actions}
        </div>
      ) : null}
    </div>
  );
}

function stopPropagation(event: React.SyntheticEvent) {
  event.stopPropagation();
}

export { TasksInboxRow };
