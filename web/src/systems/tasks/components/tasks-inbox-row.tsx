import * as React from "react";

import { StateGlyph, type StateGlyphState } from "@compozy/ui";
import { cn } from "@/lib/utils";

import type { InboxGroupId } from "../lib/inbox-grouping";

export interface TasksInboxRowProps extends Omit<React.ComponentProps<"div">, "onSelect"> {
  taskId: string;
  /** Inbox group this row belongs to (exposed as `data-group`). */
  group: InboxGroupId;
  /** Leading state glyph for the item (canonical StateGlyph mapping). */
  state: StateGlyphState;
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
 * Inbox row — 3-column grid `[ state glyph | body | meta ]`. The glyph carries
 * the item's state; unread state is expressed via the body's title weight.
 */
function TasksInboxRow({
  taskId,
  group,
  state,
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
        "grid min-h-12 items-start gap-3 border-b border-line-soft px-3 py-3 text-left transition-colors duration-base ease-out",
        trailing ? "grid-cols-[14px_minmax(0,1fr)_auto]" : "grid-cols-[14px_minmax(0,1fr)]",
        clickable &&
          "cursor-pointer hover:bg-row-hover focus-visible:outline-none focus-visible:shadow-focus-inset",
        className
      )}
      {...props}
    >
      <StateGlyph className="mt-1" data-slot="tasks-inbox-row-glyph" state={state} />

      <div className="flex min-w-0 flex-col gap-1" data-slot="tasks-inbox-row-main">
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
          className="flex shrink-0 items-center gap-1.5 self-center"
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
