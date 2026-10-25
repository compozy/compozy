import type { ReactNode } from "react";

import { Skeleton, useTopbarSlot } from "@compozy/ui";

import { cn } from "@/lib/utils";

/**
 * The window toolbar strip before the app has published its own. It goes where the app's strip
 * goes: it publishes through the window's own topbar slot, so the frame mounts it in the strip row
 * above the body (`OsWindowToolbar`, the row the loaded app's strip takes over) before the first
 * paint. Drawn inside the body instead, the body would sit one strip lower while the app loads
 * and jump up when it arrived. It renders nothing where it is used, and publishes only `toolbar`,
 * so the head keeps the window's own title and glyph; the slot is the window's own, and the
 * app's publish replaces it while this unmounting clears only what it published itself.
 *
 * The row is the same fixed-height row, rule and padding, with blocks where the controls go. The
 * blocks are presentational and hidden from assistive technology; the fallback's one status
 * region carries the loading semantics.
 */
export function ToolbarSlotSkeleton({
  leading,
  trailing,
}: {
  leading: ReactNode;
  trailing?: ReactNode;
}) {
  useTopbarSlot({
    toolbar: (
      <>
        <div
          aria-hidden="true"
          className="flex flex-1 flex-nowrap items-center gap-2.5"
          data-slot="toolbar-skeleton"
        >
          {leading}
        </div>
        {trailing ? (
          <div aria-hidden="true" className="ml-auto flex shrink-0 items-center">
            {trailing}
          </div>
        ) : null}
      </>
    ),
  });
  return null;
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
