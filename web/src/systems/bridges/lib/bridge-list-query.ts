import type { InfiniteData } from "@tanstack/react-query";

import type {
  BridgeCatalogFilter,
  BridgeHealthMap,
  BridgeListFilter,
  BridgesListResponse,
  BridgeSummary,
} from "../types";

function normalizeOptionalText(value?: string | null): string | undefined {
  if (typeof value !== "string") return undefined;
  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

export function normalizeBridgeCatalogFilter(
  filters: BridgeCatalogFilter = {}
): BridgeCatalogFilter {
  return {
    scope: filters.scope,
    workspace_id: normalizeOptionalText(filters.workspace_id),
    workspace: normalizeOptionalText(filters.workspace),
    q: normalizeOptionalText(filters.q),
    platform: normalizeOptionalText(filters.platform),
    status: filters.status,
    sort: filters.sort,
    limit: filters.limit,
  };
}

export function bridgeListRequest(filters: BridgeCatalogFilter, cursor?: string): BridgeListFilter {
  return cursor ? { ...filters, cursor } : filters;
}

export function flattenBridgePages(
  data: InfiniteData<BridgesListResponse, unknown> | undefined
): BridgeSummary[] {
  const seen = new Set<string>();
  return (
    data?.pages.flatMap(page =>
      page.bridges.filter(bridge => {
        if (seen.has(bridge.id)) return false;
        seen.add(bridge.id);
        return true;
      })
    ) ?? []
  );
}

export function bridgeHealthFromPages(
  data: InfiniteData<BridgesListResponse, unknown> | undefined
): BridgeHealthMap {
  return Object.assign({}, ...(data?.pages.map(page => page.bridge_health) ?? []));
}

export function bridgeListPage(
  data: InfiniteData<BridgesListResponse, unknown> | undefined
): BridgesListResponse | undefined {
  return data?.pages[0];
}
