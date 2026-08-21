import { useEffect } from "react";
import { useNavigate } from "@tanstack/react-router";

import { normalizeListingSearchValue } from "@/lib/listing-search";
import { useDebouncedInput } from "@/hooks/use-debounced-input";

import { useMarketplaceKind } from "./use-marketplace";
import type { MarketplaceKind, MarketplaceListing, MarketplaceRouteKind } from "../types";
import { marketplaceRouteKindFor } from "../types";
import {
  marketplaceKindPath,
  marketplaceKindScopeFromSearch,
  type MarketplaceKindScope,
  type MarketplaceKindSearch,
} from "../lib/marketplace-kind-search";
import {
  buildInstalledItems,
  type MarketplaceInstalledItem,
} from "../lib/marketplace-installed-items";
import { countMarketplaceUpdates } from "../lib/marketplace-updates";
import { useExtensionInventory } from "@/systems/extensions";
import {
  SETTINGS_QUERY_INTERVALS,
  type SettingsLayeredScope,
  type SettingsMCPServerListFilter,
  type SettingsMCPServerEntry,
  useSettingsMCPServers,
} from "@/systems/settings";
import { useSkills } from "@/systems/skill";
import { useProfileReadScope } from "@/systems/profiles";
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
  mcpConfigScope: SettingsLayeredScope;
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
  workspaceName: string | null;
  profileName: string | null;
}

export function resolveMarketplaceMCPFilter(
  profileName: string | null | undefined,
  workspaceScope: "global" | "workspace",
  activeWorkspaceId: string | null | undefined
): { filter: SettingsMCPServerListFilter; scope: SettingsLayeredScope } {
  const normalizedProfile = profileName?.trim() ?? "";
  if (normalizedProfile !== "" && normalizedProfile !== "default") {
    return {
      filter: {
        profile: normalizedProfile,
        scope: "profile",
        workspace_id: activeWorkspaceId ?? undefined,
      },
      scope: "profile",
    };
  }
  if (workspaceScope === "workspace" && activeWorkspaceId) {
    return {
      filter: { scope: "workspace", workspace_id: activeWorkspaceId },
      scope: "workspace",
    };
  }
  return { filter: { scope: "user" }, scope: "user" };
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
  const { activeWorkspace, activeWorkspaceId, scope: workspaceScope } = useActiveWorkspace();
  const { destination: profileName } = useProfileReadScope();
  const { filter: mcpFilter, scope: mcpConfigScope } = resolveMarketplaceMCPFilter(
    profileName,
    workspaceScope,
    activeWorkspaceId
  );
  const updateSearch = (updater: (current: MarketplaceKindSearch) => MarketplaceKindSearch) => {
    void navigate({
      search: current => updater(current as MarketplaceKindSearch),
      to: marketplaceKindPath(routeKind),
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
  const {
    fetchNextPage: fetchNextMarketPage,
    hasNextPage: hasNextMarketPage,
    isFetchNextPageError: hasNextMarketPageError,
    isFetchingNextPage: isFetchingNextMarketPage,
  } = marketQuery;

  const skillsQuery = useSkills(activeWorkspaceId ?? "", liveDataEnabled);
  const extensionsQuery = useExtensionInventory(liveDataEnabled);
  const mcpPollInterval = SETTINGS_QUERY_INTERVALS.collectionRefetchInterval;
  const mcpQuery = useSettingsMCPServers(mcpFilter, {
    enabled: kind === "mcp" && liveDataEnabled,
    refetchInterval: mcpPollInterval,
  });

  useEffect(() => {
    if (
      scope === "installed" &&
      liveDataEnabled &&
      hasNextMarketPage &&
      !isFetchingNextMarketPage &&
      !hasNextMarketPageError
    ) {
      void fetchNextMarketPage();
    }
  }, [
    fetchNextMarketPage,
    hasNextMarketPage,
    hasNextMarketPageError,
    isFetchingNextMarketPage,
    liveDataEnabled,
    scope,
  ]);

  const setScope = (next: MarketplaceKindScope) => {
    updateSearch(current => ({
      ...current,
      tab: next === "market" ? "market" : undefined,
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

  const mcpServers = mcpQuery.data?.mcp_servers ?? [];

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
    mcp: mcpQuery.isLoading,
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
    mcp: mcpQuery.error ?? null,
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
    marketplaceTotal,
    marketplaceTotalExact: exactMarketplaceTotal !== undefined,
    installedCount,
    updatesAvailable,
    // A partial catalog can still hide an update, so the count never claims to be complete.
    updatesAvailableExact: !marketQuery.hasNextPage && marketplaceContinuationError === null,
    marketEntries,
    installedItems: scope === "installed" ? installedItems : [],
    mcpEditorServers: mcpQuery.data?.mcp_servers ?? [],
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
      if (kind === "mcp") void mcpQuery.refetch();
    },
    workspaceId: activeWorkspaceId,
    workspaceName: activeWorkspace?.name ?? null,
    profileName: profileName === "default" ? null : profileName,
  };
}

export { useMarketplaceKindPage };
