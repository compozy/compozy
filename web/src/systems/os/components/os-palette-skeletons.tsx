import {
  MetadataList,
  Skeleton,
  SkeletonRegion,
  SkeletonText,
  type SkeletonRegionProps,
} from "@compozy/ui";

import { cn } from "@/lib/utils";

import { paletteRowClass, paletteRowTwoLineClass } from "../lib/palette-view-inset";

// The palette's async areas in their loading shape. Each is the area's twin: the same row
// frames (`palette-view-inset`), with the figures left as blocks, under one named busy region.
// Static and query-free, and none is a `CommandItem`, so nothing here registers with cmdk or
// takes the highlight.

/** What a `CommandItem` is under its row class: the flex row the loaded rows are. */
const ROW_FRAME = "flex min-h-8 w-full min-w-0 items-center rounded-md text-small-body";

export interface OsPaletteRowsSkeletonProps extends Omit<SkeletonRegionProps, "children"> {
  /** Rows to render, at most five. */
  count?: number;
}

/** One-line rows a loading area stands in with: label and trailing detail widths, varied. */
const RESULT_ROWS = [
  { id: "a", label: "w-40", detail: "w-16" },
  { id: "b", label: "w-32", detail: "w-12" },
  { id: "c", label: "w-48", detail: "w-20" },
  { id: "d", label: "w-36", detail: "w-14" },
  { id: "e", label: "w-44", detail: "w-16" },
] as const;

/** One-line rows: the glyph, the label and the quiet detail a result trails. */
export function OsPaletteRowsSkeleton({ count = 3, ...props }: OsPaletteRowsSkeletonProps) {
  return (
    <SkeletonRegion {...props}>
      {RESULT_ROWS.slice(0, count).map(row => (
        <div
          aria-hidden="true"
          className={cn(ROW_FRAME, paletteRowClass, "animate-skeleton-reveal")}
          key={row.id}
        >
          <Skeleton className="size-3.5 shrink-0 rounded-xs" />
          <SkeletonText className={cn("leading-none", row.label)} />
          <SkeletonText className={cn("ml-auto text-eyebrow", row.detail)} />
        </div>
      ))}
    </SkeletonRegion>
  );
}

/** Two-line rows: the session roundel, the title over its state line, and the time it trails. */
export function OsPaletteSessionRowsSkeleton({ count = 5, ...props }: OsPaletteRowsSkeletonProps) {
  return (
    <SkeletonRegion {...props}>
      {RESULT_ROWS.slice(0, count).map(row => (
        <div
          aria-hidden="true"
          className={cn(ROW_FRAME, paletteRowTwoLineClass, "animate-skeleton-reveal")}
          key={row.id}
        >
          <Skeleton className="size-4.5 shrink-0 rounded-full" />
          <span className="flex min-w-0 flex-col gap-1">
            <SkeletonText className={cn("leading-none font-medium", row.label)} />
            <SkeletonText className="w-28 text-eyebrow leading-snug" />
          </span>
          <SkeletonText className="ml-auto w-8 font-mono text-eyebrow" />
        </div>
      ))}
    </SkeletonRegion>
  );
}

const DETAIL_ROWS = [
  { id: "a", label: "w-16" },
  { id: "b", label: "w-12" },
  { id: "c", label: "w-20" },
] as const;

/** The detail pane while an item's detail is read: a paragraph over a few metadata rows. */
export function OsPaletteDetailSkeleton(props: Omit<SkeletonRegionProps, "children">) {
  return (
    <SkeletonRegion {...props}>
      <div aria-hidden="true">
        <SkeletonText className="text-small-body" lines={3} />
        <MetadataList className="mt-4">
          {DETAIL_ROWS.map(row => (
            <MetadataList.Row key={row.id} label={<SkeletonText className={row.label} />}>
              <SkeletonText className="w-24" />
            </MetadataList.Row>
          ))}
        </MetadataList>
      </div>
    </SkeletonRegion>
  );
}

/** Tiles of a loading grid: two rows of the widest grid (six columns). */
const GRID_TILE_IDS = ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"] as const;

export interface OsPaletteGridSkeletonProps extends Omit<SkeletonRegionProps, "children"> {
  /** The view's column count, as the loaded grid uses it. */
  columns: number;
}

/** A grid view's tiles while it is read: two rows of the real tile frame, an image over a title. */
export function OsPaletteGridSkeleton({
  columns,
  className,
  ...props
}: OsPaletteGridSkeletonProps) {
  return (
    <SkeletonRegion className={cn("max-h-72 overflow-hidden p-3", className)} {...props}>
      <div
        aria-hidden="true"
        className="grid gap-2"
        style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}
      >
        {GRID_TILE_IDS.slice(0, columns * 2).map(id => (
          <div
            className="min-w-0 animate-skeleton-reveal rounded-lg bg-card p-2 shadow-card"
            key={id}
          >
            <Skeleton className="aspect-video w-full rounded-md" />
            <div className="mt-2 flex min-w-0 items-center gap-2 text-card-title">
              <SkeletonText className="w-24" />
            </div>
          </div>
        ))}
      </div>
    </SkeletonRegion>
  );
}
