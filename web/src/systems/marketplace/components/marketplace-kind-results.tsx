import { AlertCircle, SearchX } from "lucide-react";

import { Button, Empty, Spinner } from "@agh/ui";

import { MarketplaceCard } from "./marketplace-card";
import { MarketplaceGridSkeleton } from "./marketplace-grid";
import { MarketplaceInstalledCard } from "./marketplace-installed-card";
import { useMarketplaceActionController } from "./use-marketplace-action-controller";
import type { MarketplaceKindPageModel } from "../hooks/use-marketplace-kind-page";
import { useMarketplaceMCPEditor } from "../hooks/use-marketplace-mcp-editor";
import { marketplaceKindConfig } from "../lib/marketplace-kind-config";
import type { MarketplaceKind } from "../types";

export interface MarketplaceKindResultsProps {
  actions: ReturnType<typeof useMarketplaceActionController>;
  config: ReturnType<typeof marketplaceKindConfig>;
  kind: MarketplaceKind;
  onEditMCP: ReturnType<typeof useMarketplaceMCPEditor>["openEdit"];
  page: Pick<
    MarketplaceKindPageModel,
    | "clearSearch"
    | "error"
    | "fetchNextMarketplacePage"
    | "hasNextMarketplacePage"
    | "installedItems"
    | "isFetchingNextMarketplacePage"
    | "isLoading"
    | "marketEntries"
    | "marketplaceContinuationError"
    | "query"
    | "refetch"
    | "scope"
    | "setScope"
  >;
}

function MarketplaceKindResults({
  actions,
  config,
  kind,
  onEditMCP,
  page,
}: MarketplaceKindResultsProps) {
  const hasVisibleItems =
    page.scope === "market" ? page.marketEntries.length > 0 : page.installedItems.length > 0;

  if (page.isLoading) return <MarketplaceGridSkeleton count={6} />;

  if (page.scope === "installed" && page.marketplaceContinuationError) {
    return (
      <Empty
        framed
        titleAs="h2"
        icon={AlertCircle}
        title="The marketplace catalog is incomplete"
        description="Installed status is unavailable until the complete catalog can be checked."
        cause={page.marketplaceContinuationError.message}
        action={
          <Button onClick={() => page.refetch()} size="sm" type="button">
            Retry
          </Button>
        }
      />
    );
  }

  if (page.error && !hasVisibleItems) {
    return (
      <Empty
        framed
        titleAs="h2"
        icon={AlertCircle}
        title="The marketplace is unreachable"
        description="No sources responded. Retry, or come back in a moment."
        cause={page.error.message}
        action={
          <Button onClick={() => page.refetch()} size="sm" type="button">
            Retry
          </Button>
        }
      />
    );
  }

  if (page.scope === "market" && page.marketEntries.length === 0) {
    return page.query ? (
      <Empty
        action={
          <Button onClick={page.clearSearch} size="sm" type="button" variant="outline">
            Clear search
          </Button>
        }
        data-testid={`marketplace-query-empty-${kind}`}
        description={`Nothing matches "${page.query}" in the marketplace.`}
        icon={SearchX}
        title={`No ${config.label.toLowerCase()} match this query`}
      />
    ) : (
      <Empty
        data-testid={`marketplace-empty-${kind}`}
        description={`No ${config.label.toLowerCase()} are available from configured sources.`}
        icon={AlertCircle}
        title={`No ${config.label.toLowerCase()} yet`}
      />
    );
  }

  if (page.scope === "installed" && page.installedItems.length === 0) {
    return page.query ? (
      <Empty
        action={
          <Button onClick={page.clearSearch} size="sm" type="button" variant="outline">
            Clear search
          </Button>
        }
        data-testid={`marketplace-installed-query-empty-${kind}`}
        description={`Nothing matches "${page.query}" in your ${config.installedNoun} ${config.label.toLowerCase()}.`}
        icon={SearchX}
        title={`No ${config.label.toLowerCase()} match this query`}
      />
    ) : (
      <Empty
        action={
          <Button
            data-testid={`marketplace-browse-market-${kind}`}
            onClick={() => page.setScope("market")}
            size="sm"
            type="button"
          >
            Browse the marketplace
          </Button>
        }
        data-testid={`marketplace-installed-empty-${kind}`}
        description={
          <>
            {config.teachingEmptyBody} You can also use{" "}
            <code className="rounded-xs border border-line-soft bg-input-fill px-1.5 py-px font-mono text-xs text-fg">
              {config.cliHint}
            </code>
            .
          </>
        }
        icon={config.icon}
        title={config.teachingEmptyTitle}
      />
    );
  }

  if (page.scope === "market") {
    return (
      <>
        <div
          className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3"
          data-testid="marketplace-grid"
          data-view="cards"
        >
          {page.marketEntries.map(entry => (
            <MarketplaceCard
              entry={entry}
              flashing={actions.isEntryFlashing(entry)}
              key={`${entry.kind}:${entry.entry_id}`}
              onAction={actions.handleAction}
              onFlashEnd={actions.handleFlashEnd}
              pending={actions.isEntryPending(entry)}
            />
          ))}
        </div>
        <MarketplaceContinuation page={page} />
      </>
    );
  }

  return (
    <div
      className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3"
      data-testid="marketplace-installed-grid"
      data-view="cards"
    >
      {page.installedItems.map(item => (
        <MarketplaceInstalledCard
          item={item}
          key={`${item.entry.kind}:${item.entry.entry_id}:${item.activationId ?? item.skill?.name ?? item.mcpServer?.name ?? ""}`}
          onAction={actions.handleAction}
          onAuthorize={actions.handleAuthorize}
          onDeactivate={actions.handleDeactivate}
          onEditMCP={onEditMCP}
          onRemove={actions.handleRemove}
          onToggleEnabled={actions.handleToggleEnabled}
          onUpdateBundle={actions.handleUpdateBundle}
          pending={actions.isEntryPending(item.entry) || actions.isInstalledItemPending(item)}
        />
      ))}
    </div>
  );
}

function MarketplaceContinuation({ page }: Pick<MarketplaceKindResultsProps, "page">) {
  if (page.marketplaceContinuationError) {
    return (
      <div className="flex items-center justify-center gap-3 py-3" role="alert">
        <span className="text-small-body text-danger">More results could not be loaded.</span>
        <Button onClick={page.fetchNextMarketplacePage} size="sm" type="button" variant="outline">
          Retry
        </Button>
      </div>
    );
  }

  if (!page.hasNextMarketplacePage) return null;

  return (
    <div className="flex justify-center py-3">
      <Button
        disabled={page.isFetchingNextMarketplacePage}
        onClick={page.fetchNextMarketplacePage}
        size="sm"
        type="button"
        variant="outline"
      >
        {page.isFetchingNextMarketplacePage ? (
          <Spinner aria-hidden="true" className="size-3" />
        ) : null}
        {page.isFetchingNextMarketplacePage ? "Loading…" : "Load more"}
      </Button>
    </div>
  );
}

export { MarketplaceKindResults };
