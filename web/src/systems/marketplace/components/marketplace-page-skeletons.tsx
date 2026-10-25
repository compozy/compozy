import { ListingPage, SkeletonRegion } from "@compozy/ui";

import { MarketplaceGridPlaceholder, MarketplaceGridSkeleton } from "./marketplace-grid";
import { MarketplaceInstalledShelfPlaceholder } from "./marketplace-installed-shelf-skeleton";
import {
  MARKETPLACE_INSTALLED_GRID_CLASS,
  MARKETPLACE_PAGE_BODY_CLASS,
} from "./marketplace-classes";

// Loading shapes of the two catalog pages in their listing page, for a window that shows them
// before the page's code has loaded. Each is one status region.

/** Browse before its reads answer: the installed shelf's line over the catalog grid. */
export function MarketplaceBrowseSkeleton() {
  return (
    <ListingPage data-testid="marketplace-page">
      <div className={MARKETPLACE_PAGE_BODY_CLASS}>
        <SkeletonRegion className="flex flex-col gap-3.5" label="Loading marketplace">
          <MarketplaceInstalledShelfPlaceholder />
          <MarketplaceGridPlaceholder count={8} />
        </SkeletonRegion>
      </div>
    </ListingPage>
  );
}

/** Installed before its inventory answers: one card per row from 960px. */
export function MarketplaceInstalledSkeleton() {
  return (
    <ListingPage data-testid="marketplace-installed-page">
      <div className={MARKETPLACE_PAGE_BODY_CLASS}>
        <MarketplaceGridSkeleton className={MARKETPLACE_INSTALLED_GRID_CLASS} count={4} />
      </div>
    </ListingPage>
  );
}
