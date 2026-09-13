import { RefreshCw, Store } from "lucide-react";

import { Button, ListingPage, Spinner, useTopbarSlot } from "@compozy/ui";

import type { MarketplaceSearch } from "../lib/marketplace-search";
import { MarketplaceAddMenu } from "./marketplace-add-menu";
import { MarketplaceResults } from "./marketplace-results";
import { useMarketplaceBrowse } from "./use-marketplace-browse";

interface MarketplacePageProps {
  search: MarketplaceSearch;
  liveDataEnabled?: boolean;
}

/**
 * Browse: one catalog, one kind. Head = Store · Marketplace · count · Refresh · Add ▾; strip =
 * search only; body = installed shelf → sections by source → grid.
 */
function MarketplacePage({ search, liveDataEnabled = true }: MarketplacePageProps) {
  const { actions, addMarketplace, install, page, query, strip } = useMarketplaceBrowse(
    search,
    liveDataEnabled
  );

  useTopbarSlot({
    glyph: <Store />,
    crumb: "Marketplace",
    count: page.isLoading && !page.total ? "–" : page.total,
    actions: (
      <>
        <Button
          data-testid="marketplace-refresh"
          disabled={page.isRefreshing}
          onClick={() => void page.refresh()}
          size="sm"
          type="button"
          variant="ghost"
        >
          {page.isRefreshing ? (
            <Spinner aria-hidden="true" className="size-3" />
          ) : (
            <RefreshCw aria-hidden="true" className="size-3" />
          )}
          Refresh
        </Button>
        <MarketplaceAddMenu onAddMarketplace={addMarketplace.open} onInstall={install.open} />
      </>
    ),
    toolbar: strip.toolbar,
  });

  return (
    <ListingPage data-testid="marketplace-page">
      <div className="@container flex min-w-0 flex-col gap-3.5">
        <MarketplaceResults
          actions={actions}
          onClearSearch={strip.clear}
          onInstallFromGitHub={() => install.open("github")}
          page={page}
          query={query}
        />
      </div>
      {actions.dialogs}
      {install.dialogs}
      {addMarketplace.dialog}
      <span aria-live="polite" className="sr-only">
        {query ? `Search updated · ${page.catalogItems.length} results` : ""}
      </span>
    </ListingPage>
  );
}

export { MarketplacePage };
export type { MarketplacePageProps };
