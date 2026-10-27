import {
  CatalogCardSkeleton,
  ListGroup,
  ListGroupHeader,
  ListingRowSkeleton,
  SkeletonRegion,
  SkeletonText,
  type ListingViewMode,
} from "@compozy/ui";

import { LoopCatalogLedeSkeleton } from "./loop-catalog-lede";

/** Rows (or cards) that fill a window before the read answers; the count is not yet known. */
const SKELETON_ROWS = 6;

interface LoopCatalogSkeletonProps {
  view: ListingViewMode;
  workspaceLabel?: string;
}

/**
 * The catalog before its first read: the lede and one group in the shape the view shows, so the
 * toolbar above and the rows below keep their place when the catalog arrives. It sits inside the
 * same `ListingPage` the loaded catalog does.
 */
export function LoopCatalogSkeleton({ view, workspaceLabel }: LoopCatalogSkeletonProps) {
  const groupLabel = <SkeletonText className="w-16" />;
  const groupCount = <SkeletonText className="w-3" />;
  return (
    <SkeletonRegion className="flex flex-col gap-5" label="Loading Loops">
      <LoopCatalogLedeSkeleton workspaceLabel={workspaceLabel} />
      <section>
        {view === "cards" ? (
          <>
            <ListGroupHeader
              className="border-b-0 bg-transparent px-1 pt-0"
              count={groupCount}
              label={groupLabel}
            />
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
              <CatalogCardSkeleton count={SKELETON_ROWS} />
            </div>
          </>
        ) : (
          <ListGroup
            className="overflow-hidden rounded-lg bg-card shadow-card"
            count={groupCount}
            label={groupLabel}
          >
            <ListingRowSkeleton count={SKELETON_ROWS} description trail="pill" />
          </ListGroup>
        )}
      </section>
    </SkeletonRegion>
  );
}
