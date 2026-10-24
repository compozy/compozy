import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import {
  loopAnnotationsOptions,
  loopConfigOptions,
  loopDetailOptions,
  loopRunDetailOptions,
  loopRunsListOptions,
  loopRunsOptions,
  loopsAggregateOptions,
  loopsCatalogOptions,
} from "../lib/query-options";
import { flattenLoopCatalogPages, loopCatalogPage } from "../lib/loops-list-query";
import type { LoopAggregateBreadth, LoopCatalogStableFilter, LoopRunsFilter } from "../types";
import { useProfileReadScope, type ProfileReadScope } from "@/systems/profiles";
import type { LoopBreadthTarget } from "@/systems/workspace";

/**
 * Definitions resolve for one concrete profile, so under All profiles the
 * catalog reads the acting profile's definitions ("Definitions for {profile}").
 */
export function loopDefinitionProfile(
  scope: Pick<ProfileReadScope, "aggregate" | "destination" | "params">
) {
  return scope.aggregate ? { profile: scope.destination } : scope.params;
}

export function useLoops(
  workspaceId: string,
  filters: LoopCatalogStableFilter = {},
  enabled = true
) {
  const params = loopDefinitionProfile(useProfileReadScope());
  const query = useInfiniteQuery({
    ...loopsCatalogOptions(workspaceId, { ...filters, ...params }),
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

/** The All projects definitions batch: User rows and each project's rows, bounded. */
export function useLoopsAggregate(
  breadth: LoopAggregateBreadth,
  filters: LoopCatalogStableFilter = {},
  enabled = true
) {
  const params = loopDefinitionProfile(useProfileReadScope());
  const query = useQuery(loopsAggregateOptions(breadth, { ...filters, ...params }, enabled));
  return {
    ...query,
    entries: query.data?.entries ?? [],
    errors: query.data?.errors ?? [],
    truncated: query.data?.truncated ?? false,
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
  const { params } = useProfileReadScope();
  return useQuery(loopRunsOptions(workspaceId, { ...filters, ...params }, enabled));
}

/** Runs over a breadth: one workspace's roster, or the All projects aggregate (recency). */
export function useLoopRunsList(
  target: LoopBreadthTarget,
  filters: LoopRunsFilter = {},
  enabled = true
) {
  const { params } = useProfileReadScope();
  return useQuery(loopRunsListOptions(target, { ...filters, ...params }, enabled));
}

export function useLoopRun(workspaceId: string, runId: string, enabled = true) {
  const { params } = useProfileReadScope();
  return useQuery(loopRunDetailOptions(workspaceId, runId, enabled, params));
}
