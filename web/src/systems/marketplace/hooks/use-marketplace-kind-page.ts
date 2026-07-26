import { useEffect, useRef, useState } from "react";
import { useNavigate } from "@tanstack/react-router";

import { useBundleActivations, useExtensionInventory } from "@/systems/extensions";
import {
  SETTINGS_QUERY_INTERVALS,
  useSettingsMCPServers,
  type SettingsMCPServerEntry,
} from "@/systems/settings";
import { useSkills } from "@/systems/skill";
import { useActiveWorkspace } from "@/systems/workspace";
import { normalizeListingSearchValue } from "@/lib/listing-search";

import { useMarketplaceKind } from "./use-marketplace";
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

const SEARCH_DEBOUNCE_MS = 180;

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
  mcpConfigScope: "global" | "workspace";
  setMCPConfigScope: (scope: "global" | "workspace") => void;
  marketplaceTotal: number;
  marketplaceTotalExact: boolean;
  installedCount: number;
  updatesAvailable: number;
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
  search: MarketplaceKindSearch
): MarketplaceKindPageModel {
  const navigate = useNavigate();
  const routeKind = marketplaceRouteKindFor(kind);
  const scope = marketplaceKindScopeFromSearch(search);
  const routeQuery = search.q ?? "";
  const { activeWorkspaceId } = useActiveWorkspace();
  const mcpConfigScope = search.config_scope ?? (activeWorkspaceId ? "workspace" : "global");
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [draftState, setDraftState] = useState({ routeQuery, value: routeQuery });
  const draftQuery = draftState.routeQuery === routeQuery ? draftState.value : routeQuery;

  const marketQuery = useMarketplaceKind({
    kind,
    limit: 100,
    q: scope === "market" ? routeQuery || null : null,
    workspaceId: activeWorkspaceId,
  });

  const skillsQuery = useSkills(activeWorkspaceId ?? "");
  const extensionsQuery = useExtensionInventory();
  const bundlesQuery = useBundleActivations();
  const mcpPollInterval = SETTINGS_QUERY_INTERVALS.collectionRefetchInterval;
  const mcpGlobalQuery = useSettingsMCPServers(
    { scope: "global" },
    { enabled: kind === "mcp", refetchInterval: mcpPollInterval }
  );
  const mcpWorkspaceQuery = useSettingsMCPServers(
    { scope: "workspace", workspace_id: activeWorkspaceId ?? undefined },
    {
      enabled: kind === "mcp" && Boolean(activeWorkspaceId),
      refetchInterval: mcpPollInterval,
    }
  );

  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, []);

  useEffect(() => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
      debounceRef.current = null;
    }
  }, [routeQuery]);

  useEffect(() => {
    if (
      scope === "installed" &&
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
    scope,
  ]);

  const updateSearch = (updater: (current: MarketplaceKindSearch) => MarketplaceKindSearch) => {
    void navigate({
      search: current => updater(current as MarketplaceKindSearch),
      to: `/marketplace/${routeKind}`,
    });
  };

  const setDraftQuery = (nextQuery: string) => {
    setDraftState({ routeQuery, value: nextQuery });
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      debounceRef.current = null;
      updateSearch(current => ({
        ...current,
        q: normalizeListingSearchValue(nextQuery),
      }));
    }, SEARCH_DEBOUNCE_MS);
  };

  const setScope = (next: MarketplaceKindScope) => {
    updateSearch(current => ({
      ...current,
      tab: next === "installed" ? "installed" : undefined,
    }));
  };

  const setMCPConfigScope = (next: "global" | "workspace") => {
    updateSearch(current => ({
      ...current,
      config_scope: next,
    }));
  };

  const clearSearch = () => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
      debounceRef.current = null;
    }
    setDraftState({ routeQuery, value: "" });
    updateSearch(current => ({ ...current, q: undefined }));
  };

  const marketPages = marketQuery.data?.pages ?? [];
  const marketItems = marketPages.flatMap(page => page.items);
  const exactMarketplaceTotal = marketPages.find(page => page.total !== undefined)?.total;
  const marketplaceTotal = exactMarketplaceTotal ?? marketItems.length;

  const listingBySlug = new Map<string, MarketplaceListing>();
  const listingByEntryId = new Map<string, MarketplaceListing>();
  for (const entry of marketItems) {
    listingByEntryId.set(entry.entry_id, entry);
    if (entry.install_slug) listingBySlug.set(entry.install_slug, entry);
  }

  const mcpServers = mergeMCPServers(
    mcpGlobalQuery.data?.mcp_servers ?? [],
    mcpWorkspaceQuery.data?.mcp_servers ?? []
  );

  const installedItems = buildInstalledItems({
    kind,
    query: routeQuery,
    marketItems,
    skills: skillsQuery.data ?? [],
    extensions: extensionsQuery.data ?? [],
    activations: bundlesQuery.data ?? [],
    mcpServers,
    listingBySlug,
    listingByEntryId,
  });

  const installedCount = installedItems.length;
  const updatesAvailable = installedItems.filter(item => item.entry.update_available).length;

  const marketEntries = scope === "market" ? marketItems : [];

  const inventoryLoading =
    kind === "skill"
      ? skillsQuery.isLoading
      : kind === "extension"
        ? extensionsQuery.isLoading
        : kind === "bundle"
          ? bundlesQuery.isLoading
          : kind === "mcp"
            ? mcpGlobalQuery.isLoading ||
              (Boolean(activeWorkspaceId) && mcpWorkspaceQuery.isLoading)
            : false;

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
      : marketQuery.isLoading || inventoryLoading || installedCatalogLoading;
  const inventoryError =
    kind === "skill"
      ? (skillsQuery.error ?? null)
      : kind === "extension"
        ? (extensionsQuery.error ?? null)
        : kind === "bundle"
          ? (bundlesQuery.error ?? null)
          : kind === "mcp"
            ? (mcpGlobalQuery.error ?? mcpWorkspaceQuery.error ?? null)
            : null;
  const marketError = marketplaceContinuationError ? null : marketQuery.error;

  return {
    kind,
    routeKind,
    scope,
    query: routeQuery,
    draftQuery,
    setDraftQuery,
    setScope,
    clearSearch,
    mcpConfigScope,
    setMCPConfigScope,
    marketplaceTotal,
    marketplaceTotalExact: exactMarketplaceTotal !== undefined,
    installedCount,
    updatesAvailable,
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
        : (marketplaceContinuationError ?? inventoryError ?? marketError)) ?? null,
    refetch: () => {
      if (scope === "installed" && marketplaceContinuationError) {
        void marketQuery.fetchNextPage();
        return;
      }
      void marketQuery.refetch();
      if (kind === "skill") void skillsQuery.refetch();
      if (kind === "extension") void extensionsQuery.refetch();
      if (kind === "bundle") void bundlesQuery.refetch();
      if (kind === "mcp") {
        void mcpGlobalQuery.refetch();
        if (activeWorkspaceId) void mcpWorkspaceQuery.refetch();
      }
    },
    workspaceId: activeWorkspaceId,
  };
}

export { useMarketplaceKindPage };
