import { Skeleton, SkeletonRegion, SkeletonText } from "@compozy/ui";

// The shelf's 40px line while the installed inventory loads, so the grid below does not drop when
// it arrives. Most installs have a shelf; one with nothing installed folds the line away.
const SHELF_SKELETON_CLASS = "-mx-1 flex min-h-10 items-center gap-2.5 px-1";

function MarketplaceInstalledShelfSkeletonContent() {
  return (
    <>
      <Skeleton className="size-(--size-catalog-logo) rounded-sm" />
      <span className="text-small-body font-medium">
        <SkeletonText className="w-24" />
      </span>
    </>
  );
}

/** The shelf's loading line for a page that owns one status region around everything it loads. */
function MarketplaceInstalledShelfPlaceholder() {
  return (
    <div aria-hidden="true" className={SHELF_SKELETON_CLASS}>
      <MarketplaceInstalledShelfSkeletonContent />
    </div>
  );
}

function MarketplaceInstalledShelfSkeleton() {
  return (
    <SkeletonRegion
      className={SHELF_SKELETON_CLASS}
      data-testid="marketplace-installed-shelf-skeleton"
      label="Loading installed extensions"
    >
      <MarketplaceInstalledShelfSkeletonContent />
    </SkeletonRegion>
  );
}

export { MarketplaceInstalledShelfPlaceholder, MarketplaceInstalledShelfSkeleton };
