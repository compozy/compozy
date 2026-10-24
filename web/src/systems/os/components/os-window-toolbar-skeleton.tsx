import type { ReactNode } from "react";

import { Skeleton } from "@compozy/ui";

import { cn } from "@/lib/utils";

/**
 * The window toolbar strip before the app has published its own (`OsWindowToolbar`): the same
 * fixed-height row, rule and padding, with blocks where the controls go. The loaded app mounts
 * its strip above the body and this row leaves the body in the same commit, so the body keeps
 * its place. It is presentational and hidden from assistive technology; the fallback's one status
 * region carries the loading semantics.
 */
export function OsWindowToolbarSkeleton({
  leading,
  trailing,
}: {
  leading: ReactNode;
  trailing?: ReactNode;
}) {
  return (
    <div
      aria-hidden="true"
      className="flex h-window-toolbar shrink-0 items-center gap-2.5 overflow-hidden border-b border-line bg-canvas px-3"
      data-slot="os-window-toolbar-skeleton"
    >
      <div className="flex flex-1 flex-nowrap items-center gap-2.5">{leading}</div>
      {trailing ? <div className="ml-auto flex shrink-0 items-center">{trailing}</div> : null}
    </div>
  );
}

/** The listing search field, level with the real one (`--height-search`). */
export function ToolbarSearchSkeleton({ className }: { className?: string }) {
  return (
    <Skeleton
      className={cn(
        "h-search min-w-search-input-floor basis-search-input shrink rounded-md",
        className
      )}
    />
  );
}

/** The Filter button next to the search. */
export function ToolbarFilterSkeleton() {
  return <Skeleton className="h-button-sm w-16 shrink-0 rounded-pill" />;
}

/** The rows/cards toggle at the strip's end. */
export function ToolbarViewToggleSkeleton() {
  return <Skeleton className="h-control-compact w-32 rounded-pill" />;
}

/** A segmented switch of short options (views, modes); `className` sets its width. */
export function ToolbarSegmentsSkeleton({ className = "w-40" }: { className?: string }) {
  return <Skeleton className={cn("h-control-compact shrink-0 rounded-pill", className)} />;
}
