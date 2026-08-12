import { useEffect } from "react";
import { useNavigate } from "@tanstack/react-router";

import { normalizeListingSearchValue } from "@/lib/listing-search";
import { useDebouncedInput } from "@/hooks/use-debounced-input";

import { useMarketplaceKind } from "./use-marketplace";
import type { MCPConfigScope } from "./marketplace-mcp-scope";
import type { MarketplaceKind, MarketplaceListing, MarketplaceRouteKind } from "../types";
import { marketplaceRouteKindFor } from "../types";
import {
  marketplaceKindScopeFromSearch,
  type MarketplaceKindScope,
  type MarketplaceKindSearch,
} from "../lib/marketplace-kind-search";
import {
  buildInstalledItems,
  mergeMCPServers,
  type MarketplaceInstalledItem,
} from "../lib/marketplace-installed-items";
import { countMarketplaceUpdates } from "../lib/marketplace-updates";
import { useExtensionInventory } from "@/systems/extensions";
import {
  SETTINGS_QUERY_INTERVALS,
  type SettingsMCPServerEntry,
  useSettingsMCPServers,
} from "@/systems/settings";
import { useSkills } from "@/systems/skill";
import { useActiveWorkspace } from "@/systems/workspace";

export type { MarketplaceInstalledItem } from "../lib/marketplace-installed-items";

export interface MarketplaceKindPageModel {
  kind: MarketplaceKind;
  routeKind: MarketplaceRouteKind;
  scope: MarketplaceKindScope;
  query: string;
  draftQuery: string;
  setDraftQuery: (value: string) => void;
  setScope: (scope: MarketplaceKindScope) => void;
  clearSearch: () => void;
  mcpConfigScope: MCPConfigScope;
  setMCPConfigScope: (scope: MCPConfigScope) => void;
  marketplaceTotal: number;
  marketplaceTotalExact: boolean;
  installedCount: number;
  updatesAvailable: number;
  updatesAvailableExact: boolean;
  marketEntries: readonly MarketplaceListing[];
  installedItems: readonly MarketplaceInstalledItem[];
  mcpEditorServers: readonly SettingsMCPServerEntry[];
  hasNextMarketplacePage: boolean;
  isFetchingNextMarketplacePage: boolean;
  marketplaceContinuationError: Error | null;
  fetchNextMarketplacePage: () => void;
  isLoading: boolean;
  isFetching: boolean;
  error: Error | null;
  refetch: () => void;
  workspaceId: string | null | undefined;
}

function useMarketplaceKindPage(
  kind: MarketplaceKind,
  search: MarketplaceKindSearch,
  { liveDataEnabled = true }: { liveDataEnabled?: boolean } = {}
): MarketplaceKindPageModel {
  const navigate = useNavigate();
  const routeKind = marketplaceRouteKindFor(kind);
  const scope = marketplaceKindScopeFromSearch(search);
  const routeQuery = search.q ?? "";
  const { activeWorkspaceId } = useActiveWorkspace();
  const mcpConfigScope = search.config_scope ?? (activeWorkspaceId ? "workspace" : "global");
  const updateSearch = (updater: (current: MarketplaceKindSearch) => MarketplaceKindSearch) => {
    void navigate({
      search: current => updater(current as MarketplaceKindSearch),
      to: `/marketplace/${routeKind}`,
    });
  };
  const searchInput = useDebouncedInput({
    externalValue: routeQuery,
    onCommit: nextQuery =>
      updateSearch(current => ({
        ...current,
        q: normalizeListingSearchValue(nextQuery),
      })),
  });
  const draftQuery = searchInput.draftValue;

  const marketQuery = useMarketplaceKind(
    {
      kind,
      limit: 100,
      q: scope === "market" ? routeQuery || null : null,
      workspaceId: activeWorkspaceId,
    },
    liveDataEnabled
  );

  const skillsQuery = useSkills(activeWorkspaceId ?? "", liveDataEnabled);
  const extensionsQuery = useExtensionInventory(liveDataEnabled);
  const mcpPollInterval = SETTINGS_QUERY_INTERVALS.collectionRefetchInterval;
  const mcpGlobalQuery = useSettingsMCPServers(
    { scope: "global" },
    { enabled: kind === "mcp" && liveDataEnabled, refetchInterval: mcpPollInterval }
  );
  const mcpWorkspaceQuery = useSettingsMCPServers(
    { scope: "workspace", workspace_id: activeWorkspaceId ?? undefined },
    {
      enabled: kind === "mcp" && Boolean(activeWorkspaceId) && liveDataEnabled,
      refetchInterval: mcpPollInterval,
    }
  );

  useEffect(() => {
    if (
      scope === "installed" &&
      liveDataEnabled &&
      marketQuery.hasNextPage &&
      !marketQuery.isFetchingNextPage &&
      !marketQuery.isFetchNextPageError
    ) {
      void marketQuery.fetchNextPage();
    }
  }, [
    marketQuery.fetchNextPage,
    marketQuery.hasNextPage,
    marketQuery.isFetchNextPageError,
    marketQuery.isFetchingNextPage,
    liveDataEnabled,
    scope,
  ]);

  const setScope = (next: MarketplaceKindScope) => {
    updateSearch(current => ({
      ...current,
      tab: next === "market" ? "market" : undefined,
    }));
  };

  const setMCPConfigScope = (next: MCPConfigScope) => {
    updateSearch(current => ({
      ...current,
      config_scope: next,
    }));
  };

  const clearSearch = () => {
    searchInput.clear();
  };

  const marketPages = marketQuery.data?.pages ?? [];
  const marketItems = marketPages.flatMap(page => page.items);
  const exactMarketplaceTotal = marketPages.find(page => page.total !== undefined)?.total;
  const marketplaceTotal = exactMarketplaceTotal ?? marketItems.length;

  const listingBySlug = new Map<string, MarketplaceListing>();
  const listingByEntryId = new Map<string, MarketplaceListing>();
  const listingByInstalledName = new Map<string, MarketplaceListing>();
  const listingByName = new Map<string, MarketplaceListing>();
  for (const entry of marketItems) {
    listingByEntryId.set(entry.entry_id, entry);
    if (entry.install_slug) listingBySlug.set(entry.install_slug, entry);
    if (entry.installed_name) listingByInstalledName.set(entry.installed_name, entry);
    listingByName.set(entry.name, entry);
  }

  const mcpServers = mergeMCPServers(
    mcpGlobalQuery.data?.mcp_servers ?? [],
    mcpWorkspaceQuery.data?.mcp_servers ?? []
  );

  const installedInventory = buildInstalledItems({
    kind,
    query: "",
    marketItems,
    skills: skillsQuery.data ?? [],
    extensions: extensionsQuery.data ?? [],
    mcpServers,
    listingBySlug,
    listingByEntryId,
    listingByInstalledName,
    listingByName,
  });
  const installedItems = routeQuery
    ? buildInstalledItems({
        kind,
        query: routeQuery,
        marketItems,
        skills: skillsQuery.data ?? [],
        extensions: extensionsQuery.data ?? [],
        mcpServers,
        listingBySlug,
        listingByEntryId,
        listingByInstalledName,
        listingByName,
      })
    : installedInventory;

  const installedCount = installedItems.length;
  // Counted from the complete installed projection before scope hiding, unioned with catalog
  // entries that already report an update, so the landing count is never structurally zero.
  const updatesAvailable = countMarketplaceUpdates({
    installedItems: installedInventory,
    marketItems,
  });

  const marketEntries = scope === "market" ? marketItems : [];

  const inventoryLoadingByKind = {
    extension: extensionsQuery.isLoading,
    mcp: mcpGlobalQuery.isLoading || (Boolean(activeWorkspaceId) && mcpWorkspaceQuery.isLoading),
    skill: skillsQuery.isLoading,
  } satisfies Record<MarketplaceKind, boolean>;

  const marketplaceContinuationError = marketQuery.isFetchNextPageError
    ? (marketQuery.error ?? new Error("The next marketplace page could not be loaded."))
    : null;
  const installedCatalogLoading =
    scope === "installed" &&
    !marketplaceContinuationError &&
    (marketQuery.hasNextPage || marketQuery.isFetchingNextPage);

  const isLoading =
    scope === "market"
      ? marketQuery.isLoading
      : marketQuery.isLoading || inventoryLoadingByKind[kind] || installedCatalogLoading;
  const inventoryErrorByKind = {
    extension: extensionsQuery.error ?? null,
    mcp: mcpGlobalQuery.error ?? mcpWorkspaceQuery.error ?? null,
    skill: skillsQuery.error ?? null,
  } satisfies Record<MarketplaceKind, Error | null>;
  const marketError = marketplaceContinuationError ? null : marketQuery.error;

  return {
    kind,
    routeKind,
    scope,
    query: routeQuery,
    draftQuery,
    setDraftQuery: searchInput.setDraftValue,
    setScope,
    clearSearch,
    mcpConfigScope,
    setMCPConfigScope,
    marketplaceTotal,
    marketplaceTotalExact: exactMarketplaceTotal !== undefined,
    installedCount,
    updatesAvailable,
    // A partial catalog can still hide an update, so the count never claims to be complete.
    updatesAvailableExact: !marketQuery.hasNextPage && marketplaceContinuationError === null,
    marketEntries,
    installedItems: scope === "installed" ? installedItems : [],
    mcpEditorServers: [
      ...(mcpGlobalQuery.data?.mcp_servers ?? []),
      ...(mcpWorkspaceQuery.data?.mcp_servers ?? []),
    ],
    hasNextMarketplacePage: marketQuery.hasNextPage,
    isFetchingNextMarketplacePage: marketQuery.isFetchingNextPage,
    marketplaceContinuationError,
    fetchNextMarketplacePage: () => {
      void marketQuery.fetchNextPage();
    },
    isLoading,
    isFetching: marketQuery.isFetching,
    error:
      (scope === "market"
        ? marketError
        : (marketplaceContinuationError ?? inventoryErrorByKind[kind] ?? marketError)) ?? null,
    refetch: () => {
      if (scope === "installed" && marketplaceContinuationError) {
        void marketQuery.fetchNextPage();
        return;
      }
      void marketQuery.refetch();
      if (kind === "skill") void skillsQuery.refetch();
      if (kind === "extension") void extensionsQuery.refetch();
      if (kind === "mcp") {
        void mcpGlobalQuery.refetch();
        if (activeWorkspaceId) void mcpWorkspaceQuery.refetch();
      }
    },
    workspaceId: activeWorkspaceId,
  };
}

export { useMarketplaceKindPage };
