import { AlertCircle, ClockAlert, Puzzle, SearchX, Store } from "@compozy/ui/icons";

import { Button, Empty, Skeleton, SkeletonRegion, Spinner, Time } from "@compozy/ui";
import { GithubLogo } from "@compozy/ui/logos";

import { isMarketplaceCursorStale } from "../adapters/marketplace-api-error";
import type { useMarketplacePage } from "../hooks/use-marketplace-page";
import { MarketplaceCatalogRow } from "./marketplace-catalog-row";
import { MarketplaceCategorySection } from "./marketplace-category-section";
import { MARKETPLACE_SECTION_GRID_CLASS } from "./marketplace-classes";
import { MarketplaceGrid, MarketplaceGridPlaceholder } from "./marketplace-grid";
import { MarketplaceInstalledShelf } from "./marketplace-installed-shelf";
import { MarketplaceInstalledShelfSkeleton } from "./marketplace-installed-shelf-skeleton";
import type { MarketplaceActionController } from "./use-marketplace-action-controller";

type MarketplacePageModel = ReturnType<typeof useMarketplacePage>;

interface MarketplaceResultsProps {
  actions: MarketplaceActionController;
  onClearSearch: () => void;
  onInstallFromGitHub: () => void;
  onAddMarketplace: () => void;
  page: MarketplacePageModel;
  query: string;
}

/**
 * Browse body: shelf → Featured → one section per non-empty category (no search), or the flat
 * search results, with truthful loading, stale, unreachable, empty, and continuation states from
 * the daemon envelope.
 */
function MarketplaceResults({
  actions,
  onClearSearch,
  onInstallFromGitHub,
  onAddMarketplace,
  page,
  query,
}: MarketplaceResultsProps) {
  const shelf = page.isInstalledLoading ? (
    <MarketplaceInstalledShelfSkeleton />
  ) : (
    <MarketplaceInstalledShelf
      items={page.installedItems}
      query={query}
      updates={page.updates.length}
    />
  );

  if (page.isLoading) {
    return (
      <>
        {shelf}
        <MarketplaceResultsSkeleton searching={query !== ""} />
      </>
    );
  }

  const hasItems = page.hasItems;

  if (page.catalogError && !hasItems) {
    return (
      <>
        {shelf}
        <Empty
          action={
            <Button
              data-testid="marketplace-retry"
              variant="secondary"
              onClick={() => void page.refresh()}
              size="sm"
              type="button"
            >
              Retry
            </Button>
          }
          cause={page.catalogError.message}
          data-testid="marketplace-unreachable"
          description="No sources responded. Retry, or come back in a moment."
          framed
          icon={AlertCircle}
          title="The marketplace is unreachable"
          titleAs="h2"
        />
      </>
    );
  }

  if (!hasItems) {
    return (
      <>
        {shelf}
        {query ? (
          <Empty
            action={
              <Button onClick={onClearSearch} size="sm" type="button" variant="secondary">
                Clear search
              </Button>
            }
            data-testid="marketplace-query-empty"
            description="Try a different word."
            icon={SearchX}
            title={`No results for "${query}"`}
          />
        ) : (
          <Empty
            action={
              <>
                <Button onClick={onAddMarketplace} size="sm" type="button">
                  <Store aria-hidden="true" />
                  Add plugin marketplace…
                </Button>
                <Button onClick={onInstallFromGitHub} size="sm" type="button" variant="ghost">
                  <GithubLogo aria-hidden="true" className="size-3.5" />
                  Install from GitHub…
                </Button>
              </>
            }
            data-testid="marketplace-empty"
            description="No extensions are available from your sources. Add a plugin marketplace to list its plugins here, or install one directly from GitHub."
            icon={Puzzle}
            title="No extensions yet"
          />
        )}
      </>
    );
  }

  const inventory = { items: page.installedItems, pending: page.isInstalledLoading };

  return (
    <>
      {shelf}
      {page.stale ? <MarketplaceStaleLine page={page} /> : null}
      {query ? (
        <MarketplaceGrid data-testid="marketplace-search-results">
          {page.catalogItems.map(entry => (
            <MarketplaceCatalogRow
              actions={actions}
              entry={entry}
              inventory={inventory}
              key={`${entry.source_ref ?? entry.source}:${entry.entry_id}`}
              query={query}
            />
          ))}
        </MarketplaceGrid>
      ) : (
        <div className="flex min-w-0 flex-col gap-8" data-testid="marketplace-sections">
          {page.sections.map(section => (
            <MarketplaceCategorySection
              actions={actions}
              inventory={inventory}
              key={section.id}
              section={section}
            />
          ))}
        </div>
      )}
      <MarketplaceContinuation page={page} />
    </>
  );
}

const SECTION_SKELETONS = ["section-skeleton-1", "section-skeleton-2", "section-skeleton-3"];

/** Loading: three sections of six rows on the home, one result grid under a search. */
function MarketplaceResultsSkeleton({ searching }: { searching: boolean }) {
  return (
    <SkeletonRegion label="Loading marketplace entries">
      {searching ? (
        <MarketplaceGridPlaceholder count={8} />
      ) : (
        <div className="flex min-w-0 flex-col gap-8">
          {SECTION_SKELETONS.map(id => (
            <div className="flex flex-col gap-3" key={id}>
              <Skeleton className="h-5 w-32" />
              <MarketplaceGridPlaceholder className={MARKETPLACE_SECTION_GRID_CLASS} count={6} />
            </div>
          ))}
        </div>
      )}
    </SkeletonRegion>
  );
}

/** The last projection the daemon could load, under one 12px line that says so — never a banner. */
/** Attribute failed refreshes and cached timestamps to the degraded sources reported by the daemon. */
function MarketplaceStaleLine({ page }: { page: MarketplacePageModel }) {
  const degraded = page.sources.filter(source => source.state === "degraded");
  const failed = degraded.length > 0 || Boolean(page.diagnostic);
  const lastRead = (failed ? degraded : page.sources)
    .map(source => source.last_read_at)
    .filter((value): value is string => typeof value === "string" && value !== "")
    .sort()
    .at(-1);
  return (
    <p
      className="flex items-center gap-2 text-eyebrow text-subtle"
      data-testid="marketplace-stale"
      role="status"
    >
      {failed ? (
        <ClockAlert aria-hidden="true" className="size-3 shrink-0 text-warning" />
      ) : (
        <Spinner aria-hidden="true" className="size-3 shrink-0" />
      )}
      <span>
        {failed ? (
          <>
            {degraded.length > 0
              ? `Could not refresh ${degraded.map(source => source.name).join(", ")}.`
              : "Could not refresh the catalog."}
            {degraded.some(source => source.count > 0) ? (
              <>
                {" "}
                Showing cached entries
                {lastRead ? (
                  <>
                    {" "}
                    from <Time iso={lastRead} />
                  </>
                ) : null}
                .
              </>
            ) : null}
          </>
        ) : (
          <>
            Refreshing the catalog from {lastRead ? <Time iso={lastRead} /> : "the last refresh"}…
          </>
        )}
      </span>
      {failed ? (
        <Button
          disabled={page.isRefreshing}
          onClick={() => void page.refresh()}
          size="xs"
          type="button"
          variant="ghost"
        >
          Retry
        </Button>
      ) : null}
    </p>
  );
}

function MarketplaceContinuation({ page }: { page: MarketplacePageModel }) {
  if (page.catalogError) {
    const restarting = isMarketplaceCursorStale(page.catalogError);
    return (
      <div className="flex items-center justify-center gap-3 py-3" role="alert">
        <span className="text-small-body text-destructive">
          {restarting
            ? "The catalog changed while loading; showing the last complete catalog."
            : "The catalog could not be refreshed."}
        </span>
        <Button onClick={() => void page.refresh()} size="sm" type="button" variant="secondary">
          Retry
        </Button>
      </div>
    );
  }

  return null;
}

export { MarketplaceResults };
export type { MarketplaceResultsProps };
