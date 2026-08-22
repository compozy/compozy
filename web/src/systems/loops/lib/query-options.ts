import { infiniteQueryOptions, queryOptions } from "@tanstack/react-query";

import { listLoopNodes } from "../adapters/loop-nodes-api";
import { getLoopRequest, listLoopRequests } from "../adapters/loop-requests-api";
import {
  getLoopRunBriefing,
  getLoopRunRoster,
  getLoopRunTimeline,
} from "../adapters/loop-run-reads-api";
import { diffLoopRun } from "../adapters/loop-timetravel-api";
import {
  getLoop,
  getLoopAnnotations,
  getLoopConfig,
  getLoopRun,
  listLoopRuns,
  listLoops,
} from "../adapters/loops-api";
import { isTerminalLoopStatus } from "./loop-formatters";
import { type LoopNodeInventoryStableFilter, loopsKeys } from "./query-keys";
import { loopCatalogRequest, normalizeLoopCatalogFilter } from "./loops-list-query";
import type {
  LoopCatalogStableFilter,
  LoopDiffQuery,
  LoopRequestStableFilter,
  LoopRosterStableFilter,
  LoopRunsFilter,
  LoopTimelineStableFilter,
} from "../types";

const DEFAULT_STALE_TIME = 15_000;
const DEFAULT_REFETCH_INTERVAL = 30_000;
const LIVE_STALE_TIME = 5_000;
const LIVE_REFETCH_INTERVAL = 15_000;
/**
 * Roster and timeline page sizes. Both sit inside the daemon's 1-500 range. The
 * roster is deliberately generous: the DAG draws the whole topology, and a run
 * with fewer than 200 node x round rows — nearly all of them — reads in one page.
 */
const ROSTER_PAGE_LIMIT = 200;
const TIMELINE_PAGE_LIMIT = 50;

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

/**
 * The served verdict. The page renders it; it never recomputes a different one
 * (Safety Invariant 12). A terminal run's briefing is immutable, so polling stops
 * with the run — the same rule `loopRunDetailOptions` follows.
 */
export function loopRunBriefingOptions(workspaceId: string, runId: string, enabled = true) {
  return queryOptions({
    queryKey: loopsKeys.runBriefing(workspaceId, runId),
    queryFn: ({ signal }) => getLoopRunBriefing(workspaceId, runId, signal),
    staleTime: LIVE_STALE_TIME,
    refetchInterval: query =>
      isTerminalLoopStatus(query.state.data?.status) ? false : LIVE_REFETCH_INTERVAL,
    enabled: Boolean(workspaceId) && Boolean(runId) && enabled,
  });
}

/**
 * The complete node × round roster — healthy nodes included, which is precisely
 * what the lifecycle projection cannot give (it skips nodes with no control,
 * wait, or retry). The DAG, the roster table, and the step list all read this
 * one page set, so they cannot drift from each other.
 *
 * Fan-out items page under `next_cursor`; `fanout_rollups` arrives on every page,
 * so a wide fan-out renders its counts without fetching a single item.
 */
export function loopRunRosterOptions(
  workspaceId: string,
  runId: string,
  filters: LoopRosterStableFilter = {},
  enabled = true
) {
  const normalizedFilters = { ...filters, limit: filters.limit ?? ROSTER_PAGE_LIMIT };
  return infiniteQueryOptions({
    queryKey: loopsKeys.runRoster(workspaceId, runId, normalizedFilters),
    queryFn: ({ pageParam, signal }) =>
      getLoopRunRoster(workspaceId, runId, { ...normalizedFilters, cursor: pageParam }, signal),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: page => page.next_cursor || undefined,
    staleTime: LIVE_STALE_TIME,
    refetchInterval: query =>
      isTerminalLoopStatus(query.state.data?.pages.at(-1)?.run_status)
        ? false
        : LIVE_REFETCH_INTERVAL,
    enabled: Boolean(workspaceId) && Boolean(runId) && enabled,
  });
}

/**
 * The durable story. The first page is the NEWEST window and carries `head_seq`;
 * older history pages backward on demand through an opaque cursor fenced to that
 * head, so appends never shift a page set out from under the reader.
 *
 * Deliberately unpolled: the SSE stream is the live channel (it resumes at
 * `head_seq`), and re-reading a newest-window page on a timer would fight the
 * fence. Reads reconcile on stream lifecycle events instead (ADR-005) — and a
 * window regaining focus is not a lifecycle event, so it never re-anchors a
 * fenced page set either.
 */
export function loopRunTimelineOptions(
  workspaceId: string,
  runId: string,
  filters: LoopTimelineStableFilter = {},
  enabled = true
) {
  const normalizedFilters = {
    ...filters,
    view: filters.view ?? "notable",
    limit: filters.limit ?? TIMELINE_PAGE_LIMIT,
  };
  return infiniteQueryOptions({
    queryKey: loopsKeys.runTimeline(workspaceId, runId, normalizedFilters),
    queryFn: ({ pageParam, signal }) =>
      getLoopRunTimeline(workspaceId, runId, { ...normalizedFilters, cursor: pageParam }, signal),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: page => page.next_cursor || undefined,
    staleTime: LIVE_STALE_TIME,
    refetchOnWindowFocus: false,
    enabled: Boolean(workspaceId) && Boolean(runId) && enabled,
  });
}

export function loopRequestsOptions(
  workspaceId: string,
  filters: LoopRequestStableFilter = {},
  enabled = true
) {
  const normalizedFilters = {
    ...filters,
    state: filters.state ?? "pending",
    limit: filters.limit ?? 50,
  };
  return infiniteQueryOptions({
    queryKey: loopsKeys.requests(workspaceId, normalizedFilters),
    queryFn: ({ pageParam, signal }) =>
      listLoopRequests(workspaceId, { ...normalizedFilters, cursor: pageParam }, signal),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: page => page.next_cursor || undefined,
    staleTime: LIVE_STALE_TIME,
    refetchInterval: LIVE_REFETCH_INTERVAL,
    enabled: Boolean(workspaceId) && enabled,
  });
}

export function loopRequestAttentionOptions(
  workspaceId: string,
  enabled = true,
  refetchInterval: number | false = LIVE_REFETCH_INTERVAL
) {
  return queryOptions({
    queryKey: loopsKeys.requestAttention(workspaceId),
    queryFn: ({ signal }) => listLoopRequests(workspaceId, { state: "pending", limit: 50 }, signal),
    staleTime: LIVE_STALE_TIME,
    refetchInterval,
    enabled: Boolean(workspaceId) && enabled,
  });
}

export function loopRequestDetailOptions(
  workspaceId: string,
  runId: string,
  generation: number,
  nodeId: string,
  itemIndex?: number,
  enabled = true
) {
  return queryOptions({
    queryKey: loopsKeys.requestDetail(workspaceId, runId, generation, nodeId, itemIndex),
    queryFn: ({ signal }) =>
      getLoopRequest({ workspaceId, runId, generation, nodeId, itemIndex }, signal),
    staleTime: LIVE_STALE_TIME,
    enabled: Boolean(workspaceId) && Boolean(runId) && generation > 0 && Boolean(nodeId) && enabled,
  });
}

export function loopRunDiffOptions(
  workspaceId: string,
  runId: string,
  query: LoopDiffQuery = {},
  enabled = true
) {
  return queryOptions({
    queryKey: loopsKeys.runDiff(workspaceId, runId, query),
    queryFn: ({ signal }) => diffLoopRun({ workspaceId, runId }, query, signal),
    staleTime: LIVE_STALE_TIME,
    refetchInterval: settled => {
      const data = settled.state.data;
      if (!data) return LIVE_REFETCH_INTERVAL;
      const bothTerminal =
        isTerminalLoopStatus(data.base.status) && isTerminalLoopStatus(data.against.status);
      return bothTerminal ? false : LIVE_REFETCH_INTERVAL;
    },
    enabled: Boolean(workspaceId) && Boolean(runId) && enabled,
  });
}
