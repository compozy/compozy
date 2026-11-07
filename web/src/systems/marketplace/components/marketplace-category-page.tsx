import { Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useRef } from "react";
import { AlertCircle, LayoutGrid, SearchX } from "@compozy/ui/icons";

import { Button, Empty, ListingPage, Spinner, useTopbarSlot } from "@compozy/ui";

import { useMarketplaceCategoryPage } from "../hooks/use-marketplace-category-page";
import type { MarketplaceSearch } from "../lib/marketplace-search";
import { MarketplaceCatalogRow } from "./marketplace-catalog-row";
import { MARKETPLACE_PAGE_BODY_CLASS, MARKETPLACE_SECTION_GRID_CLASS } from "./marketplace-classes";
import { MarketplaceGrid, MarketplaceGridSkeleton } from "./marketplace-grid";
import { useMarketplaceActionController } from "./use-marketplace-action-controller";
import { useMarketplaceSearchStrip } from "./use-marketplace-search-strip";

interface MarketplaceCategoryPageProps {
  category: string;
  search: MarketplaceSearch;
  liveDataEnabled?: boolean;
}

/**
 * One category (US-002): crumb Marketplace › {label} with the daemon's count, a search scoped to
 * the category, then every entry in daemon order, paged as the end of the list comes into view.
 */
function MarketplaceCategoryPage({
  category,
  search,
  liveDataEnabled = true,
}: MarketplaceCategoryPageProps) {
  const query = search.q ?? "";
  const navigate = useNavigate();
  const page = useMarketplaceCategoryPage(category, query, liveDataEnabled);
  const actions = useMarketplaceActionController();
  const label = page.label ?? category;
  const strip = useMarketplaceSearchStrip({
    category,
    label: `Search ${label}`,
    placeholder: `Search ${label}…`,
    query,
    testId: "marketplace-category-search",
    to: "/marketplace",
  });
  const back = () => void navigate({ search: {}, to: "/marketplace" });

  useTopbarSlot({
    onBack: back,
    crumbs: [{ id: "marketplace", label: "Marketplace", onSelect: back }],
    crumb: page.unknownCategory ? "Not found" : label,
    count: page.unknownCategory ? undefined : page.isLoading ? "–" : page.total,
    toolbar: page.unknownCategory ? undefined : strip.toolbar,
  });

  return (
    <ListingPage data-category={category} data-testid="marketplace-category-page">
      <div className={MARKETPLACE_PAGE_BODY_CLASS}>
        <MarketplaceCategoryBody
          actions={actions}
          category={category}
          label={label}
          onClearSearch={strip.clear}
          page={page}
          query={query}
        />
      </div>
      {actions.dialogs}
      <span aria-live="polite" className="sr-only" data-testid="marketplace-live-region">
        <span>{query && !page.isLoading ? `Search updated · ${page.total} results` : ""}</span>
        <span>{actions.announcement}</span>
      </span>
    </ListingPage>
  );
}

function MarketplaceCategoryBody({
  actions,
  category,
  label,
  onClearSearch,
  page,
  query,
}: {
  actions: ReturnType<typeof useMarketplaceActionController>;
  category: string;
  label: string;
  onClearSearch: () => void;
  page: ReturnType<typeof useMarketplaceCategoryPage>;
  query: string;
}) {
  if (page.unknownCategory) {
    return (
      <Empty
        action={
          <Button
            render={<Link search={{}} to="/marketplace" />}
            size="sm"
            type="button"
            variant="secondary"
          >
            Back to Marketplace
          </Button>
        }
        data-testid="marketplace-category-not-found"
        description="Browse every category from the Marketplace."
        framed
        icon={LayoutGrid}
        title="This category doesn't exist"
        titleAs="h2"
      />
    );
  }
  if (page.isLoading) {
    return <MarketplaceGridSkeleton className={MARKETPLACE_SECTION_GRID_CLASS} count={8} />;
  }
  if (page.error && page.items.length === 0) {
    return (
      <Empty
        action={
          <Button onClick={page.retry} size="sm" type="button" variant="secondary">
            Retry
          </Button>
        }
        cause={page.error.message}
        data-testid="marketplace-category-error"
        description="The category could not be loaded."
        framed
        icon={AlertCircle}
        title="The marketplace is unreachable"
        titleAs="h2"
      />
    );
  }
  if (page.items.length === 0) {
    return (
      <Empty
        action={
          query ? (
            <Button onClick={onClearSearch} size="sm" type="button" variant="secondary">
              Clear search
            </Button>
          ) : undefined
        }
        data-testid="marketplace-category-empty"
        description={query ? "Try a different word." : undefined}
        icon={SearchX}
        title={query ? `No results for "${query}" in ${label}` : `Nothing in ${label} yet`}
      />
    );
  }
  return (
    <>
      <MarketplaceGrid
        className={MARKETPLACE_SECTION_GRID_CLASS}
        data-testid={`marketplace-category-grid-${category}`}
      >
        {page.items.map(entry => (
          <MarketplaceCatalogRow
            actions={actions}
            category={category}
            entry={entry}
            inventory={page.inventory}
            key={`${entry.source_ref ?? entry.source}:${entry.entry_id}`}
            query={query}
          />
        ))}
      </MarketplaceGrid>
      {page.error ? (
        <div className="flex items-center justify-center gap-3 py-3" role="alert">
          <span className="text-small-body text-destructive">
            The next entries could not be loaded.
          </span>
          <Button onClick={page.retryMore} size="sm" type="button" variant="secondary">
            Retry
          </Button>
        </div>
      ) : page.hasMore ? (
        <MarketplaceLoadMore loading={page.isLoadingMore} onLoadMore={page.loadMore} />
      ) : null}
    </>
  );
}

/**
 * The end of the list: it asks for the next page once it scrolls into view, and stays a real
 * button for keyboards and for windows without IntersectionObserver.
 */
function MarketplaceLoadMore({
  loading,
  onLoadMore,
}: {
  loading: boolean;
  onLoadMore: () => Promise<void>;
}) {
  const sentinel = useRef<HTMLDivElement>(null);
  const latestLoadMore = useRef(onLoadMore);
  useEffect(() => {
    latestLoadMore.current = onLoadMore;
  });
  // One observer for the sentinel's lifetime; it always asks with the current loader.
  useEffect(() => {
    const node = sentinel.current;
    if (!node || typeof IntersectionObserver === "undefined") return;
    const observer = new IntersectionObserver(entries => {
      if (entries.some(entry => entry.isIntersecting)) void latestLoadMore.current();
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, []);
  return (
    <div
      className="flex justify-center py-3"
      data-testid="marketplace-category-more"
      ref={sentinel}
    >
      <Button
        disabled={loading}
        onClick={() => void onLoadMore()}
        size="sm"
        type="button"
        variant="ghost"
      >
        {loading ? <Spinner aria-hidden="true" className="size-3.5" /> : null}
        {loading ? "Loading…" : "Load more"}
      </Button>
    </div>
  );
}

export { MarketplaceCategoryPage };
export type { MarketplaceCategoryPageProps };
