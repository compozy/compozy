import type { ReactNode } from "react";

import { Checkbox } from "@compozy/ui";

import { cn } from "@/lib/utils";

import { useSessionsBoardGroupSelection } from "../../hooks/use-sessions-board-view-selection";

/**
 * A column's or lane's select-all, in the leading slot of its header (the
 * `group/board-select` element): the header's mark gives way to it on hover or
 * focus and while a selection is open, as a list row's badge gives way to its
 * checkbox. It covers the cards the column or lane shows; clearing it removes
 * only those. Without a selection or cards, the mark stays alone.
 */
export function SessionsBoardSelectAll({
  ids,
  label,
  testId,
  children,
}: {
  /** The cards on screen, in order. */
  ids: readonly string[];
  /** What the checkbox covers, as the operator reads it ("Marketing, Working"). */
  label: string;
  testId: string;
  /** The mark the checkbox replaces; a lane header has none. */
  children?: ReactNode;
}) {
  const group = useSessionsBoardGroupSelection(ids);
  if (!group) return children ?? null;
  const all = group.state === "all";
  return (
    <span className="relative grid size-4 shrink-0 place-items-center">
      {children ? (
        <span
          className={cn(
            "grid place-items-center transition-opacity duration-fast group-focus-within/board-select:opacity-0 group-hover/board-select:opacity-0 motion-reduce:transition-none",
            group.mode && "opacity-0"
          )}
        >
          {children}
        </span>
      ) : null}
      <Checkbox
        aria-label={all ? `Deselect all in ${label}` : `Select all in ${label}`}
        checked={all}
        className={cn(
          "absolute inset-0 transition-opacity duration-fast motion-reduce:transition-none",
          group.mode
            ? "opacity-100"
            : "pointer-events-none opacity-0 group-focus-within/board-select:pointer-events-auto group-focus-within/board-select:opacity-100 group-hover/board-select:pointer-events-auto group-hover/board-select:opacity-100"
        )}
        data-testid={testId}
        indeterminate={group.state === "some"}
        onCheckedChange={group.toggle}
      />
    </span>
  );
}
