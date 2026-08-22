import type { InfiniteData } from "@tanstack/react-query";

import { normalizeOptionalText } from "../adapters/loops-api-errors";
import type {
  LoopCatalogEntry,
  LoopCatalogFilter,
  LoopCatalogStableFilter,
  LoopsListResponse,
} from "../types";

export function normalizeLoopCatalogFilter(
  filters: LoopCatalogStableFilter = {}
): LoopCatalogStableFilter {
  return {
    q: normalizeOptionalText(filters.q),
    kind: filters.kind,
    category: normalizeOptionalText(filters.category),
    status: filters.status,
    sort: filters.sort,
    limit: filters.limit,
  };
}

export function loopCatalogRequest(
  filters: LoopCatalogStableFilter,
  cursor?: string
): LoopCatalogFilter {
  return cursor ? { ...filters, cursor } : filters;
}

export function flattenLoopCatalogPages(
  data: InfiniteData<LoopsListResponse, unknown> | undefined
): LoopCatalogEntry[] {
  const seen = new Set<string>();
  return (
    data?.pages.flatMap(page =>
      page.loops.filter(loop => {
        if (seen.has(loop.name)) return false;
        seen.add(loop.name);
        return true;
      })
    ) ?? []
  );
}

export function loopCatalogPage(
  data: InfiniteData<LoopsListResponse, unknown> | undefined
): LoopsListResponse | undefined {
  return data?.pages[0];
}
