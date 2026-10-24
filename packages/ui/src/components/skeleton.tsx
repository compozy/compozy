import * as React from "react";

import { skeletonSlots } from "../lib/skeleton-slots";
import { cn } from "../lib/utils";

// Compositor-only motion lives in tokens-runtime.css: `skeleton-shimmer` sweeps a
// pseudo-element (transform) and `animate-skeleton-reveal` fades the block in after a
// short delay (opacity). The host clips the sweep, hence `relative overflow-hidden`.
const SKELETON_CLASS =
  "relative overflow-hidden rounded-md bg-surface-2 animate-skeleton-reveal skeleton-shimmer";

/**
 * One placeholder block. It lays out immediately but stays transparent for a short beat and
 * then fades in, so a load that settles quickly never flashes a placeholder and a skeleton that
 * mirrors real geometry reserves its space with no layout shift. Motion is compositor-only and
 * stills under reduced motion. Compose the shape primitives (`SkeletonText`, `ListingRowSkeleton`,
 * …) rather than sizing bare blocks by hand wherever a real primitive's geometry exists.
 *
 * A block carries no semantics: wrap the whole loading area in `SkeletonRegion`.
 */
function Skeleton({ className, ...props }: React.ComponentProps<"div">) {
  return <div data-slot="skeleton" className={cn(SKELETON_CLASS, className)} {...props} />;
}

export interface SkeletonRegionProps extends React.ComponentProps<"div"> {
  /** Visually hidden name for the loading area, e.g. "Loading providers". Defaults to "Loading". */
  label?: string;
}

/**
 * The loading semantics for a skeleton area, in one place: a polite status region marked busy,
 * named by visually hidden text. Wrap exactly one around everything that stands in for the
 * pending content (rows, cards, a whole table) and do not nest them. The skeleton shapes inside
 * are presentational and hidden from assistive technology, so the area is announced once rather
 * than once per placeholder. `DataSurface.Loading` renders this when given skeleton children.
 *
 * The label comes first so `:last-child` selectors on the placeholder rows (the last row drops
 * its divider) still hold.
 */
function SkeletonRegion({ label = "Loading", className, children, ...props }: SkeletonRegionProps) {
  return (
    <div
      data-slot="skeleton-region"
      role="status"
      aria-busy="true"
      className={className}
      {...props}
    >
      <span className="sr-only">{label}</span>
      {children}
    </div>
  );
}

export interface SkeletonTextProps extends React.ComponentProps<"span"> {
  /** Lines of text to stand in for. Each is exactly one line box tall. Defaults to 1. */
  lines?: number;
  /** Width of the last line as a fraction of the line (0 to 1) when there is more than one. Defaults to 0.6. */
  lastLineWidth?: number;
}

/**
 * Text placeholder sized by the type it replaces. Every line is one line box tall (`1lh`), so it
 * takes the line-height of whatever typography it sits in and cannot drift from the real text:
 * place it inside the real slot (`ListingRow.Title`, `ItemDescription`, a table cell) and the
 * slot supplies the type, or pass the text utility yourself (`className="text-small-body"`).
 * The bar inside each line is shorter than the line box, the way glyphs sit in it.
 *
 * Spans throughout, so it is valid inside `<p>`, `<b>` and `<h3>`. Width is the caller's: a
 * fixed width (`w-32`) in a shrink-wrapped parent such as a flex item, where a percentage would
 * collapse to nothing, or nothing in a block parent to fill it. It never outgrows its parent.
 */
function SkeletonText({ lines = 1, lastLineWidth = 0.6, className, ...props }: SkeletonTextProps) {
  const slots = skeletonSlots("skeleton-text-line", lines);
  const lastWidth = `${Math.min(1, Math.max(0, lastLineWidth)) * 100}%`;

  return (
    <span
      aria-hidden="true"
      data-slot="skeleton-text"
      className={cn("block max-w-full", className)}
      {...props}
    >
      {slots.map(slot => (
        <span key={slot.id} data-slot="skeleton-text-line" className="flex h-lh items-center">
          <span
            data-slot="skeleton"
            className={cn(SKELETON_CLASS, "block h-[0.6lh] w-full")}
            style={
              slot.position > 0 && slot.position === lines - 1 ? { width: lastWidth } : undefined
            }
          />
        </span>
      ))}
    </span>
  );
}

export interface SkeletonRowsProps extends React.ComponentProps<"div"> {
  count?: number;
  rowClassName?: string;
  children?: React.ReactNode;
}

/**
 * Generic stacked rows: `children` repeated `count` times, or three plain lines. For a list that
 * has a real counterpart prefer the matching shape primitive (`ListingRowSkeleton`, …), whose
 * rows reuse the real row so geometry cannot drift. Wrap in `SkeletonRegion` for semantics.
 */
function SkeletonRows({
  count = 3,
  rowClassName,
  className,
  children,
  ...props
}: SkeletonRowsProps) {
  const rows = skeletonSlots("skeleton-row", count);

  return (
    <div data-slot="skeleton-rows" className={cn("flex flex-col", className)} {...props}>
      {rows.map(row => (
        <div
          data-slot="skeleton-row"
          className={cn("flex flex-col gap-2", rowClassName)}
          key={row.id}
        >
          {children ?? (
            <>
              <Skeleton className="h-3.5 w-2/3" />
              <Skeleton className="h-3 w-full" />
              <Skeleton className="h-3 w-3/4" />
            </>
          )}
        </div>
      ))}
    </div>
  );
}

export { Skeleton, SkeletonRegion, SkeletonRows, SkeletonText };
