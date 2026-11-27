import { cn } from "@/lib/utils";

import type {
  SessionDragHandlers,
  SessionDropPosition,
  SessionRowDropEdge,
} from "../../hooks/use-session-list-dnd";

/** The drag source/target frame; it draws the drop line on the pending edge. */
export function SessionRowDragFrame({
  drag,
  dropEdge,
  dragging,
  dropTestId,
  children,
  className,
  ...rest
}: React.ComponentProps<"div"> & {
  drag: SessionDragHandlers | undefined;
  dropEdge: SessionRowDropEdge | null;
  dragging: boolean;
  dropTestId: string;
}) {
  return (
    <div
      {...rest}
      {...drag}
      data-dragging={dragging || undefined}
      data-drop={dropEdge ?? undefined}
      className={cn(
        dragging && "opacity-50",
        // Dropping onto the row nests the dragged session under it.
        dropEdge === "into" && "rounded-md bg-brand-tint ring-1 ring-brand ring-inset",
        className
      )}
    >
      {dropEdge === "before" || dropEdge === "after" ? (
        <SessionRowDropLine edge={dropEdge} testId={dropTestId} />
      ) : null}

      {children}
    </div>
  );
}

/** The drop indicator on the edge where a dragged session will land. */
function SessionRowDropLine({ edge, testId }: { edge: SessionDropPosition; testId: string }) {
  return (
    <span
      aria-hidden="true"
      data-testid={testId}
      className={cn(
        "pointer-events-none absolute inset-x-1 z-10 h-0.5 rounded-full bg-brand",
        edge === "before" ? "-top-px" : "-bottom-px"
      )}
    />
  );
}
