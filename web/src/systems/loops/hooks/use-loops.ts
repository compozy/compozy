import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import {
  loopAnnotationsOptions,
  loopConfigOptions,
  loopDetailOptions,
  loopRunDetailOptions,
  loopRunsOptions,
  loopsCatalogOptions,
} from "../lib/query-options";
import { flattenLoopCatalogPages, loopCatalogPage } from "../lib/loops-list-query";
import type { LoopCatalogStableFilter, LoopRunsFilter } from "../types";

export function useLoops(
  workspaceId: string,
  filters: LoopCatalogStableFilter = {},
  enabled = true
) {
  const query = useInfiniteQuery({
    ...loopsCatalogOptions(workspaceId, filters),
    enabled: Boolean(workspaceId) && enabled,
  });
  const page = loopCatalogPage(query.data);

  return {
    ...query,
    loops: flattenLoopCatalogPages(query.data),
    facets: page?.facets,
    total: page?.page.total ?? 0,
  };
}

export function useLoop(workspaceId: string, name: string, enabled = true) {
  return useQuery(loopDetailOptions(workspaceId, name, enabled));
}

export function useLoopConfig(workspaceId: string, name: string, enabled = true) {
  const query = useQuery(loopConfigOptions(workspaceId, name, enabled));
  return {
    ...query,
    data: query.data?.config ?? null,
    effectiveConfig: query.data?.effectiveConfig,
  };
}

export function useLoopAnnotations(workspaceId: string, name: string, enabled = true) {
  return useQuery(loopAnnotationsOptions(workspaceId, name, enabled));
}

export function useLoopRuns(workspaceId: string, filters: LoopRunsFilter = {}, enabled = true) {
  return useQuery(loopRunsOptions(workspaceId, filters, enabled));
}

export function useLoopRun(workspaceId: string, runId: string, enabled = true) {
  return useQuery(loopRunDetailOptions(workspaceId, runId, enabled));
}
