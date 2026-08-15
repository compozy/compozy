import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import { listLoopNodes } from "../adapters/loop-nodes-api";
import {
  getLoop,
  getLoopAnnotations,
  getLoopConfig,
  getLoopRun,
  listGoalTurns,
  listLoopRuns,
  listLoops,
} from "../adapters/loops-api";
import { isTerminalLoopStatus } from "./loop-formatters";
import { type LoopNodeInventoryStableFilter, loopsKeys } from "./query-keys";
import { loopCatalogRequest, normalizeLoopCatalogFilter } from "./loops-list-query";
import type { GoalTurnFilter, LoopCatalogStableFilter, LoopRunsFilter } from "../types";

const DEFAULT_STALE_TIME = 15_000;
const DEFAULT_REFETCH_INTERVAL = 30_000;
const LIVE_STALE_TIME = 5_000;
const LIVE_REFETCH_INTERVAL = 15_000;

export function loopsCatalogOptions(workspaceId: string, filters: LoopCatalogStableFilter = {}) {
  const normalizedFilters = normalizeLoopCatalogFilter(filters);
  return infiniteQueryOptions({
    queryKey: loopsKeys.catalog(workspaceId, normalizedFilters),
    queryFn: ({ pageParam, signal }) =>
      listLoops(workspaceId, loopCatalogRequest(normalizedFilters, pageParam), signal),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: lastPage => (lastPage.page.has_more ? lastPage.page.next_cursor : undefined),
    staleTime: DEFAULT_STALE_TIME,
  });
}

export function goalTurnsOptions(
  workspaceId: string,
  runId: string,
  filters: Pick<GoalTurnFilter, "node" | "item" | "limit"> = {}
) {
  const normalizedFilters = { ...filters, limit: filters.limit ?? 50 };
  return infiniteQueryOptions({
    queryKey: loopsKeys.goalTurns(workspaceId, runId, normalizedFilters),
    queryFn: ({ pageParam, signal }) =>
      listGoalTurns(workspaceId, runId, { ...normalizedFilters, after_seq: pageParam }, signal),
    initialPageParam: 0,
    getNextPageParam: page => page.next_after_seq ?? undefined,
  });
}

/**
 * One inventory view. The server owns filtering, ordering (oldest in state
 * first), and the cursor, so the page never presents a loaded slice as the whole
 * truth — `has_more` is `next_cursor`'s presence and nothing is counted locally.
 */
export function loopNodeInventoryOptions(
  workspaceId: string,
  filters: LoopNodeInventoryStableFilter,
  enabled = true
) {
  const normalizedFilters = { ...filters, limit: filters.limit ?? 50 };
  return infiniteQueryOptions({
    queryKey: loopsKeys.nodeInventory(workspaceId, normalizedFilters),
    queryFn: ({ pageParam, signal }) =>
      listLoopNodes(workspaceId, { ...normalizedFilters, cursor: pageParam }, signal),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: page => page.next_cursor || undefined,
    staleTime: LIVE_STALE_TIME,
    refetchInterval: LIVE_REFETCH_INTERVAL,
    enabled: Boolean(workspaceId) && Boolean(normalizedFilters.state) && enabled,
  });
}

const EXISTS_STALE_TIME = 30_000;

/** Cheap presence check for one inventory state — one item, no paging, no count. */
export function loopNodeExistsOptions(
  workspaceId: string,
  state: "waiting" | "attention",
  enabled = true
) {
  return queryOptions({
    queryKey: loopsKeys.nodeExists(workspaceId, state),
    queryFn: ({ signal }) => listLoopNodes(workspaceId, { state, limit: 1 }, signal),
    staleTime: EXISTS_STALE_TIME,
    refetchOnWindowFocus: true,
    enabled: Boolean(workspaceId) && enabled,
  });
}

export function loopDetailOptions(workspaceId: string, name: string, enabled = true) {
  return queryOptions({
    queryKey: loopsKeys.detail(workspaceId, name),
    queryFn: ({ signal }) => getLoop(workspaceId, name, signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: DEFAULT_REFETCH_INTERVAL,
    enabled: Boolean(workspaceId) && Boolean(name) && enabled,
  });
}

export function loopConfigOptions(workspaceId: string, name: string, enabled = true) {
  return queryOptions({
    queryKey: loopsKeys.config(workspaceId, name),
    queryFn: ({ signal }) => getLoopConfig(workspaceId, name, signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: DEFAULT_REFETCH_INTERVAL,
    enabled: Boolean(workspaceId) && Boolean(name) && enabled,
  });
}

export function loopAnnotationsOptions(workspaceId: string, name: string, enabled = true) {
  return queryOptions({
    queryKey: loopsKeys.annotations(workspaceId, name),
    queryFn: ({ signal }) => getLoopAnnotations(workspaceId, name, signal),
    staleTime: DEFAULT_STALE_TIME,
    refetchInterval: DEFAULT_REFETCH_INTERVAL,
    enabled: Boolean(workspaceId) && Boolean(name) && enabled,
  });
}

export function loopRunsOptions(workspaceId: string, filters: LoopRunsFilter = {}, enabled = true) {
  return queryOptions({
    queryKey: loopsKeys.runs(workspaceId, filters),
    queryFn: ({ signal }) => listLoopRuns(workspaceId, filters, signal),
    staleTime: LIVE_STALE_TIME,
    refetchInterval: LIVE_REFETCH_INTERVAL,
    enabled: Boolean(workspaceId) && enabled,
  });
}

export function loopRunDetailOptions(workspaceId: string, runId: string, enabled = true) {
  return queryOptions({
    queryKey: loopsKeys.runDetail(workspaceId, runId),
    queryFn: ({ signal }) => getLoopRun(workspaceId, runId, signal),
    staleTime: LIVE_STALE_TIME,
    // Poll only while the run is live; a terminal run's projection is immutable, so
    // the run page stops refetching once it reaches a terminal state (contract-lane
    // risk, task-18 review) instead of polling a finished run forever.
    refetchInterval: query =>
      isTerminalLoopStatus(query.state.data?.run.status) ? false : LIVE_REFETCH_INTERVAL,
    enabled: Boolean(workspaceId) && Boolean(runId) && enabled,
  });
}
