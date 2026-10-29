import { Skeleton, SkeletonRegion, SkeletonText, cn } from "@compozy/ui";

export interface ComposerPopoverRowsSkeletonProps {
  /** The status region's name, e.g. "Loading monitors". */
  label: string;
  "data-testid"?: string;
  /** Rows the list expects; at least one, at most `PLACEHOLDER_ROW_POSITIONS`. */
  count: number;
  /** The live row's frame and text column, so placeholders and rows cannot drift. */
  rowClassName: string;
  textClassName: string;
  /** Widths cycled per row for the title line and the meta line. */
  titleWidths: readonly string[];
  metaWidths: readonly string[];
}

/** The most placeholder rows a composer popover holds for a list it has not read yet. */
const PLACEHOLDER_ROW_POSITIONS = [0, 1, 2, 3] as const;

/**
 * Placeholder rows for a composer popover list (monitors, processes): the
 * state glyph, a title over its meta line, and the row action's place, as the
 * one status region of the area.
 */
export function ComposerPopoverRowsSkeleton({
  label,
  "data-testid": testId,
  count,
  rowClassName,
  textClassName,
  titleWidths,
  metaWidths,
}: ComposerPopoverRowsSkeletonProps) {
  const rows = PLACEHOLDER_ROW_POSITIONS.slice(
    0,
    Math.min(Math.max(1, count), PLACEHOLDER_ROW_POSITIONS.length)
  );
  return (
    <SkeletonRegion label={label} data-testid={testId} className="flex flex-col gap-1">
      <ul aria-hidden="true" className="flex flex-col gap-0.5">
        {rows.map(position => (
          <li key={position} className={rowClassName}>
            <Skeleton className="mt-0.75 size-3 shrink-0 rounded-full" />
            <span className={textClassName}>
              <SkeletonText
                className={cn("text-transcript-meta", titleWidths[position % titleWidths.length])}
              />
              <SkeletonText
                className={cn("text-transcript-caption", metaWidths[position % metaWidths.length])}
              />
            </span>
            <span className="flex size-button-icon-xs shrink-0 items-center justify-center">
              <Skeleton className="size-3 rounded-sm" />
            </span>
          </li>
        ))}
      </ul>
    </SkeletonRegion>
  );
}
