import { Link } from "@tanstack/react-router";
import { AlertCircle, Puzzle, RefreshCw, SearchX } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

import { Button, Empty, ListingPage, Spinner, StatusDot, useTopbarSlot } from "@compozy/ui";

import type { InstalledExtensionView } from "@/systems/extensions";

import { useUpdateMarketplaceExtensions } from "../hooks/use-marketplace-actions";
import type { useMarketplaceInstalledPage } from "../hooks/use-marketplace-page";
import {
  formatExtensionContents,
  installedDetailSearch,
  installedDisplayName,
  installedEntryId,
  installedExtensionKey,
  installedLogoEntry,
  installedOriginWord,
  installedScopeWord,
} from "../lib/marketplace-installed-view";
import type { MarketplaceSearch } from "../lib/marketplace-search";
import { useAddMarketplaceDialog } from "./use-add-marketplace-dialog";
import { MarketplaceAddMenu } from "./marketplace-add-menu";
import { MarketplaceEntryCard } from "./marketplace-entry-card";
import { MarketplaceInstalledTrail } from "./marketplace-entry-trail";
import { MarketplaceGrid, MarketplaceGridSkeleton } from "./marketplace-grid";
import { formatMarketplaceVersion, marketplaceErrorMessage } from "./marketplace-ui";
import type { useMarketplaceActionController } from "./use-marketplace-action-controller";
import { useMarketplaceInstalled } from "./use-marketplace-installed";

interface MarketplaceInstalledPageProps {
  search: MarketplaceSearch;
  liveDataEnabled?: boolean;
}

/**
 * Installed: the drill-in from the shelf. One flat list of everything installed with contents
 * after the name and the management controls on the row; an updates line above when n > 0.
 */
function MarketplaceInstalledPage({
  search,
  liveDataEnabled = true,
}: MarketplaceInstalledPageProps) {
  const { actions, backToBrowse, install, page, query, strip } = useMarketplaceInstalled(
    search,
    liveDataEnabled
  );

  const addMarketplace = useAddMarketplaceDialog();

  useTopbarSlot({
    onBack: backToBrowse,
    crumbs: [{ id: "marketplace", label: "Marketplace", onSelect: backToBrowse }],
    crumb: "Installed",
    count: page.isPending ? "–" : page.installedCount,
    actions: (
      <>
        <Button
          data-testid="marketplace-installed-refresh"
          disabled={page.isFetching}
          onClick={() => void page.refetch()}
          size="sm"
          type="button"
          variant="ghost"
        >
          {page.isFetching ? (
            <Spinner aria-hidden="true" className="size-3" />
          ) : (
            <RefreshCw aria-hidden="true" className="size-3" />
          )}
          Refresh
        </Button>
        <MarketplaceAddMenu onInstall={install.open} onAddMarketplace={addMarketplace.open} />
      </>
    ),
    toolbar: strip.toolbar,
  });

  return (
    <ListingPage data-testid="marketplace-installed-page">
      <div className="@container flex min-w-0 flex-col gap-3.5">
        {page.updates.length > 0 ? (
          <MarketplaceUpdatesLine actions={actions} updates={page.updates} />
        ) : null}
        <MarketplaceInstalledBody
          actions={actions}
          liveDataEnabled={liveDataEnabled}
          onClearSearch={strip.clear}
          page={page}
          query={query}
        />
      </div>
      {actions.dialogs}
      {install.dialogs}
      {addMarketplace.dialog}
      <span aria-live="polite" className="sr-only">
        {query ? `Search updated · ${page.items.length} results` : ""}
      </span>
    </ListingPage>
  );
}

type InstalledPageModel = ReturnType<typeof useMarketplaceInstalledPage>;

function MarketplaceInstalledBody({
  actions,
  liveDataEnabled,
  onClearSearch,
  page,
  query,
}: {
  actions: ReturnType<typeof useMarketplaceActionController>;
  liveDataEnabled: boolean;
  onClearSearch: () => void;
  page: InstalledPageModel;
  query: string;
}) {
  if (page.isPending) return <MarketplaceGridSkeleton count={4} />;

  if (page.error && page.installedCount === 0) {
    return (
      <Empty
        action={
          <Button onClick={() => void page.refetch()} size="sm" type="button">
            Retry
          </Button>
        }
        cause={page.error.message}
        data-testid="marketplace-installed-error"
        description="The installed inventory could not be loaded."
        framed
        icon={AlertCircle}
        title="Installed extensions are unavailable"
        titleAs="h2"
      />
    );
  }

  if (page.items.length === 0) {
    return query ? (
      <Empty
        action={
          <Button onClick={onClearSearch} size="sm" type="button" variant="outline">
            Clear search
          </Button>
        }
        data-testid="marketplace-installed-query-empty"
        description={`Nothing matches "${query}" in your installed extensions.`}
        icon={SearchX}
        title="No installed extensions match this query"
      />
    ) : (
      <Empty
        action={
          <Button
            data-testid="marketplace-browse"
            nativeButton={false}
            render={<Link search={{}} to="/marketplace" />}
            size="sm"
          >
            Browse the marketplace
          </Button>
        }
        data-testid="marketplace-installed-empty"
        description={
          <>
            Everything you install from the marketplace shows up here. You can also use{" "}
            <code className="rounded-xs border border-line-soft bg-input-fill px-1.5 py-px font-mono text-xs text-fg">
              compozy extension install &lt;slug&gt;
            </code>
            .
          </>
        }
        icon={Puzzle}
        title="No extensions installed yet"
      />
    );
  }

  return (
    <MarketplaceGrid data-testid="marketplace-installed-grid">
      {page.items.map(item => {
        const pending = actions.isItemPending(item);
        return (
          <MarketplaceEntryCard
            contents={formatExtensionContents(item.extension.contents)}
            data-testid={`marketplace-installed-card-${item.extension.name}`}
            description={item.listing?.description}
            entry={installedLogoEntry(item)}
            flashing={actions.isItemFlashing(item)}
            key={installedExtensionKey(item)}
            link={{
              params: { entryId: installedEntryId(item) },
              search: { ...installedDetailSearch(item), from: "installed", q: query || undefined },
              to: "/marketplace/$entryId",
            }}
            onFlashEnd={() => actions.endItemFlash(item)}
            pending={pending}
            title={installedDisplayName(item)}
            trail={
              <MarketplaceInstalledTrail
                item={item}
                liveDataEnabled={liveDataEnabled}
                onToggleEnabled={actions.toggleEnabled}
                onUpdate={actions.updateInstalled}
                pending={pending}
                query={query}
              />
            }
            words={[installedOriginWord(item), installedScopeWord(item.extension)].filter(
              (word): word is string => word !== null
            )}
          />
        );
      })}
    </MarketplaceGrid>
  );
}

function updateSummary(item: InstalledExtensionView): string {
  const from = formatMarketplaceVersion(item.extension.version);
  const to = formatMarketplaceVersion(item.listing?.version ?? item.extension.remote_version);
  const name = installedDisplayName(item);
  return from && to ? `${name} ${from} → ${to}` : name;
}

/**
 * Updates line: warning dot, the count, the names when n ≤ 2, and Update all through the batch
 * endpoint. The daemon answers per extension; each failure is named while the rest land.
 */
function MarketplaceUpdatesLine({
  actions,
  updates,
}: {
  actions: ReturnType<typeof useMarketplaceActionController>;
  updates: readonly InstalledExtensionView[];
}) {
  const batch = useUpdateMarketplaceExtensions();
  const [inFlight, setInFlight] = useState<ReadonlySet<string>>(() => new Set());
  const count = updates.length;
  const label = `${count} ${count === 1 ? "update" : "updates"} available`;

  const updateAll = async () => {
    const names = updates.map(item => item.extension.name);
    setInFlight(new Set(names));
    try {
      const result = await actions.trackInstalledItems(updates, () => batch.mutateAsync({ names }));
      const failed = result.updates.filter(update => update.error || update.status === "failed");
      for (const update of failed) {
        toast.error(`${update.name}: ${update.error?.message ?? "update failed"}`);
      }
      const landed = result.updates.length - failed.length;
      if (landed > 0) {
        toast.success(`${landed} of ${names.length} updated`);
      }
    } catch (error) {
      toast.error(marketplaceErrorMessage(error, "Failed to update extensions"));
    } finally {
      setInFlight(new Set());
    }
  };

  const busy = batch.isPending || inFlight.size > 0;
  const pendingSomewhere = updates.some(item => actions.isItemPending(item));

  return (
    <div
      className="-mx-1 flex min-h-8 items-center gap-2 px-1 text-eyebrow text-muted"
      data-testid="marketplace-updates-line"
    >
      <StatusDot aria-hidden="true" size="sm" tone="warning" />
      <b className="font-medium text-fg" data-testid="marketplace-updates-count">
        {label}
      </b>
      {busy ? (
        <span>Updating {inFlight.size}…</span>
      ) : count <= 2 ? (
        <span className="min-w-0 truncate">{updates.map(updateSummary).join(" · ")}</span>
      ) : null}
      <Button
        className="ml-auto"
        data-testid="marketplace-update-all"
        disabled={busy || pendingSomewhere}
        onClick={() => void updateAll()}
        size="xs"
        type="button"
        variant="ghost"
      >
        {busy ? <Spinner aria-hidden="true" className="size-3" /> : null}
        {busy ? "Updating…" : "Update all"}
      </Button>
    </div>
  );
}

export { MarketplaceInstalledPage };
export type { MarketplaceInstalledPageProps };
