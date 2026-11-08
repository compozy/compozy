import { useNavigate } from "@tanstack/react-router";
import { CircleAlert } from "@compozy/ui/icons";

import { Button } from "@compozy/ui";

import type { InstalledExtensionView } from "@/systems/extensions";
import {
  deriveMCPAuthFilter,
  MCPAuthorizeDialog,
  useMCPDefinitionAuthorization,
  type UseMCPAuthorizeReturn,
} from "@/systems/settings";

import { useMarketplaceExtensionMCPServer } from "../hooks/use-marketplace-detail-mcp-server";
import { catalogDetailSearch } from "../lib/marketplace-installed-view";
import {
  findInstalledItem,
  installedServerFailure,
  installedSetupNeeds,
  marketplaceSetupState,
} from "../lib/marketplace-setup-state";
import type { MarketplaceCatalogListing } from "../types";
import { MarketplaceEntryCard } from "./marketplace-entry-card";
import { MarketplaceCatalogTrail } from "./marketplace-entry-trail";
import type { MarketplaceActionController } from "./use-marketplace-action-controller";

/** The installed inventory a row reads its setup state from. */
interface MarketplaceRowInventory {
  items: readonly InstalledExtensionView[];
  pending: boolean;
}

interface MarketplaceCatalogRowProps {
  entry: MarketplaceCatalogListing;
  actions: MarketplaceActionController;
  inventory: MarketplaceRowInventory;
  query?: string;
  /** Sections drop the description; search and the category page keep it. */
  compact?: boolean;
  /** Category page the row lives on, so the detail's Back returns there. */
  category?: string;
  /**
   * The one category the surrounding list shows (a category section or page): the row drops its
   * category word, which would only repeat the heading. Featured and search mix categories.
   */
  listCategory?: string;
}

/**
 * Community entries keep their tier word; everything else names its category (US-003), except in
 * a list of that one category, where the word would repeat the heading.
 */
function rowWords(entry: MarketplaceCatalogListing, listCategory?: string): string[] {
  const tier = entry.tier?.trim();
  if (tier && tier !== "official" && tier !== "unverified") {
    const author = entry.author?.trim();
    return [author ? `${tier} · ${author}` : tier];
  }
  const label = entry.category_label?.trim();
  return label && entry.category !== listCategory ? [label] : [];
}

/**
 * One catalog row with its single action (US-007): Add · Adding… · Set up · Ready · Turn on ·
 * Couldn't turn on + Retry, read from the listing and the installed inventory, never guessed.
 */
function MarketplaceCatalogRow({
  entry,
  actions,
  inventory,
  query = "",
  compact = false,
  category,
  listCategory = category,
}: MarketplaceCatalogRowProps) {
  const item = inventory.pending ? null : findInstalledItem(entry, inventory.items);
  const failure = actions.enableFailureFor(entry);
  const pending = actions.isEntryPending(entry);
  const state = marketplaceSetupState({
    entry,
    item,
    inventoryPending: inventory.pending,
    pending,
    enableFailed: failure !== null,
  });
  const detailSearch = {
    ...catalogDetailSearch(entry),
    q: query || undefined,
    ...(category ? { category } : {}),
  };
  // Owned by the row, not by Set up: a confirmed sign-in refreshes the inventory and turns Set up
  // into Ready, which must not unmount the dialog before the operator reads it and presses Done.
  const authorization = useMCPDefinitionAuthorization();
  return (
    <>
      <MarketplaceEntryCard
        compact={compact}
        data-testid={`marketplace-card-${entry.entry_id}`}
        description={entry.description}
        entry={entry}
        flashing={actions.isEntryFlashing(entry)}
        link={{
          params: { entryId: entry.entry_id },
          search: detailSearch,
          to: "/marketplace/$entryId",
        }}
        onFlashEnd={() => actions.endEntryFlash(entry)}
        pending={pending}
        query={query}
        trail={
          <MarketplaceCatalogTrail
            entry={entry}
            onInstall={actions.install}
            onUpdate={actions.update}
            pending={pending}
            setup={{
              state,
              enableCause: failure?.cause.error ?? null,
              enabling: actions.isEntryEnabling(entry),
              onRetry: () => actions.retryEnable(entry),
              serverFailure: item ? installedServerFailure(item) : null,
              onTurnOn: () => {
                if (!item) return;
                actions.turnOn(entry, {
                  name: item.extension.name,
                  profileName: item.extension.profile,
                  workspaceId: item.extension.workspace_id ?? null,
                });
              },
              setUpAction:
                state === "setup" && item ? (
                  <MarketplaceSetUpAction
                    authorize={authorization.authorize}
                    detailSearch={detailSearch}
                    entry={entry}
                    item={item}
                  />
                ) : null,
            }}
          />
        }
        unverified={entry.trust?.registry_tier === "unverified"}
        words={rowWords(entry, listCategory)}
      />
      <MCPAuthorizeDialog
        authorize={authorization.authorize}
        scope={authorization.scope}
        server={authorization.server}
      />
    </>
  );
}

/**
 * Set up (US-006): one server waiting for sign-in starts that sign-in here; a missing input or
 * several servers open the detail with `?setup=1`, which focuses what is left.
 */
function MarketplaceSetUpAction({
  authorize,
  entry,
  item,
  detailSearch,
}: {
  authorize: Pick<UseMCPAuthorizeReturn, "requestAuthorize">;
  entry: MarketplaceCatalogListing;
  item: InstalledExtensionView;
  detailSearch: ReturnType<typeof catalogDetailSearch> & { q?: string; category?: string };
}) {
  const navigate = useNavigate();
  const needs = installedSetupNeeds(item);
  const target =
    needs.missingInputs.length === 0 && needs.needsAuthorization.length === 1
      ? needs.needsAuthorization[0]
      : undefined;
  // Read in the background, never awaited: Set up is usable from the inventory right away, and a
  // press before the live server answers opens the detail's setup instead of waiting on it.
  const live = useMarketplaceExtensionMCPServer(target, target !== undefined);
  const setUp = () => {
    const server = live.server;
    const filter = server ? deriveMCPAuthFilter(server) : null;
    if (target && server && filter) {
      authorize.requestAuthorize(filter, server);
      return;
    }
    void navigate({
      params: { entryId: entry.entry_id },
      search: { ...detailSearch, setup: 1 },
      to: "/marketplace/$entryId",
    });
  };
  return (
    <Button
      aria-label={`Set up ${entry.name}`}
      data-testid={`marketplace-setup-${entry.entry_id}`}
      onClick={setUp}
      size="sm"
      type="button"
      variant="secondary"
    >
      <CircleAlert aria-hidden="true" className="size-3.5 text-warning" />
      Set up
    </Button>
  );
}

export { MarketplaceCatalogRow };
export type { MarketplaceCatalogRowProps, MarketplaceRowInventory };
