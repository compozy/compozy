import { AlertCircle, ClockAlert, Puzzle, SearchX, Store } from "lucide-react";

import { Button, Empty, Spinner, Time } from "@compozy/ui";
import { GithubLogo } from "@compozy/ui/logos";

import { isMarketplaceCursorStale } from "../adapters/marketplace-api-error";
import type { useMarketplacePage } from "../hooks/use-marketplace-page";
import { catalogDetailSearch } from "../lib/marketplace-installed-view";
import type { MarketplaceCatalogListing } from "../types";
import { MarketplaceCatalogSection } from "./marketplace-catalog-section";
import { MarketplaceEntryCard } from "./marketplace-entry-card";
import { MarketplaceCatalogTrail } from "./marketplace-entry-trail";
import { MarketplaceGrid, MarketplaceGridSkeleton } from "./marketplace-grid";
import { MarketplaceInstalledShelf } from "./marketplace-installed-shelf";
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

/** Community word after the name: tier and author for anything the catalog does not mark official. */
function originWords(entry: MarketplaceCatalogListing): string[] {
  const tier = entry.tier?.trim();
  if (!tier || tier === "official" || tier === "unverified") return [];
  const author = entry.author?.trim();
  return [author ? `${tier} · ${author}` : tier];
}

/**
 * Browse body: shelf → sections by source (only with two or more) → grid, with truthful loading,
 * stale, unreachable, empty, and continuation states from the daemon envelope.
 */
function MarketplaceResults({
  actions,
  onClearSearch,
  onInstallFromGitHub,
  onAddMarketplace,
  page,
  query,
}: MarketplaceResultsProps) {
  const shelf = (
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
        <MarketplaceGridSkeleton count={6} />
      </>
    );
  }

  const hasItems = page.catalogItems.length > 0;

  if (page.catalogError && !hasItems) {
    return (
      <>
        {shelf}
        <Empty
          action={
            <Button
              data-testid="marketplace-retry"
              variant="outline"
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
              <Button onClick={onClearSearch} size="sm" type="button" variant="outline">
                Clear search
              </Button>
            }
            data-testid="marketplace-query-empty"
            description={`Nothing matches "${query}" in the marketplace.`}
            icon={SearchX}
            title="No extensions match this query"
          />
        ) : (
          <Empty
            action={
              <>
                <Button onClick={onAddMarketplace} size="sm" type="button" variant="neutral">
                  <Store aria-hidden="true" className="size-3" />
                  Add plugin marketplace…
                </Button>
                <Button onClick={onInstallFromGitHub} size="sm" type="button" variant="ghost">
                  <GithubLogo aria-hidden="true" className="size-3" />
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

  const renderCard = (entry: MarketplaceCatalogListing) => (
    <MarketplaceEntryCard
      data-testid={`marketplace-card-${entry.entry_id}`}
      description={entry.description}
      query={query}
      entry={entry}
      flashing={actions.isEntryFlashing(entry)}
      key={`${entry.source_ref ?? entry.source}:${entry.entry_id}`}
      link={{
        params: { entryId: entry.entry_id },
        search: { ...catalogDetailSearch(entry), q: query || undefined },
        to: "/marketplace/$entryId",
      }}
      onFlashEnd={() => actions.endEntryFlash(entry)}
      pending={actions.isEntryPending(entry)}
      trail={
        <MarketplaceCatalogTrail
          entry={entry}
          onInstall={actions.install}
          onUpdate={actions.update}
          pending={actions.isEntryPending(entry)}
        />
      }
      unverified={entry.trust?.registry_tier === "unverified"}
      words={originWords(entry)}
    />
  );

  // Sections need two or more sources that list something: a zero-plugin source never earns a
  // header, and a query that narrows to one section keeps its header so the count stays readable.
  const sectioned = page.catalogSections.filter(section => section.count > 0).length >= 2;

  return (
    <>
      {shelf}
      {page.stale ? <MarketplaceStaleLine page={page} /> : null}
      {sectioned ? (
        page.catalogSections.map(section => {
          if (section.items.length === 0) return null;
          return (
            <MarketplaceCatalogSection
              count={query ? section.items.length : section.count}
              gist={<MarketplaceSectionGist query={query} section={section} />}
              key={section.name}
              name={section.name}
            >
              <MarketplaceGrid data-testid={`marketplace-grid-${section.name}`}>
                {section.items.map(renderCard)}
              </MarketplaceGrid>
            </MarketplaceCatalogSection>
          );
        })
      ) : (
        <MarketplaceGrid>{page.catalogItems.map(renderCard)}</MarketplaceGrid>
      )}
      <MarketplaceContinuation page={page} />
    </>
  );
}

type MarketplaceCatalogSectionModel = MarketplacePageModel["catalogSections"][number];

/**
 * What the summary says after the count: a degraded source names its last read and that it could
 * not refresh; a query names matches against the source's authoritative total; otherwise the kind
 * of source it is. Nothing here invents a success the envelope did not report.
 */
function MarketplaceSectionGist({
  query,
  section,
}: {
  query: string;
  section: MarketplaceCatalogSectionModel;
}) {
  if (section.state === "degraded") {
    return (
      <span data-testid={`marketplace-section-${section.name}-degraded`}>
        last read {section.last_read_at ? <Time iso={section.last_read_at} /> : "unknown"} · could
        not refresh
      </span>
    );
  }
  if (query) {
    return `${section.items.length} of ${section.count} matches “${query}”`;
  }
  if (section.kind === "preset") return "Plugin marketplace";
  if (section.kind === "custom") return "Plugin marketplace · custom";
  return null;
}

/** The last projection the daemon could load, under one 12px line that says so — never a banner. */
function MarketplaceStaleLine({ page }: { page: MarketplacePageModel }) {
  const failed =
    page.sources.some(source => source.state === "degraded") || Boolean(page.diagnostic);
  const lastRead = page.sources
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
        {failed ? "Showing the catalog from " : "Refreshing the catalog from "}
        {lastRead ? <Time iso={lastRead} /> : "the last refresh"}
        {failed ? " — the sources did not answer." : "…"}
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
        <span className="text-small-body text-danger">
          {restarting
            ? "The catalog changed while loading; showing the last complete catalog."
            : "The catalog could not be refreshed."}
        </span>
        <Button onClick={() => void page.refresh()} size="sm" type="button" variant="outline">
          Retry
        </Button>
      </div>
    );
  }

  return null;
}

export { MarketplaceResults };
export type { MarketplaceResultsProps };
