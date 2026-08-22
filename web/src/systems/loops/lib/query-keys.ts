import { normalizeOptionalText } from "../adapters/loops-api-errors";
import { normalizeLoopCatalogFilter } from "./loops-list-query";
import type {
  LoopCatalogStableFilter,
  LoopDiffQuery,
  LoopNodeInventoryFilter,
  LoopRequestStableFilter,
  LoopRosterStableFilter,
  LoopRunsFilter,
  LoopStreamFilter,
  LoopTimelineStableFilter,
} from "../types";

/** Inventory filter minus the continuation cursor, which lives in `pageParam`. */
export type LoopNodeInventoryStableFilter = Omit<LoopNodeInventoryFilter, "cursor">;

/**
 * A key segment, trimmed exactly as the request will be.
 *
 * Derived from the canonical normalizer rather than re-implementing its trim:
 * a whitespace-padded filter must never key a cache entry separate from the
 * identical request it produces, and two hand-written trims are two chances for
 * that to stop being true. Keys need a string, so an absent value collapses to
 * empty here instead of `undefined`.
 */
function normalizeText(value?: string | null): string {
  return normalizeOptionalText(value) ?? "";
}

function normalizeNumber(value?: number): string {
  return value === undefined ? "" : String(value);
}

function normalizeBoolean(value?: boolean): boolean | "" {
  return value === undefined ? "" : value;
}

/**
 * Hierarchical Loop query keys. Every read is workspace-scoped (the Loop API is
 * mounted under `/api/workspaces/{workspace_id}`), so `workspace_id` participates
 * in every key: a workspace switch never serves another workspace's cache, and
 * SSE/mutation invalidation can target a single loop, a workspace, or everything.
 */
export const loopsKeys = {
  all: ["loops"] as const,

  catalogRoot: () => [...loopsKeys.all, "catalog"] as const,
  catalogByWorkspace: (workspaceId: string) => [...loopsKeys.catalogRoot(), workspaceId] as const,
  catalog: (workspaceId: string, filters: LoopCatalogStableFilter = {}) => {
    const normalized = normalizeLoopCatalogFilter(filters);
    return [
      ...loopsKeys.catalogByWorkspace(workspaceId),
      normalizeText(normalized.q),
      normalizeText(normalized.kind),
      normalizeText(normalized.category),
      normalizeText(normalized.status),
      normalizeText(normalized.sort),
      normalizeNumber(normalized.limit),
    ] as const;
  },

  details: () => [...loopsKeys.all, "detail"] as const,
  detail: (workspaceId: string, name: string) =>
    [...loopsKeys.details(), workspaceId, name] as const,

  configRoot: () => [...loopsKeys.all, "config"] as const,
  config: (workspaceId: string, name: string) =>
    [...loopsKeys.configRoot(), workspaceId, name] as const,

  annotationsRoot: () => [...loopsKeys.all, "annotations"] as const,
  annotations: (workspaceId: string, name: string) =>
    [...loopsKeys.annotationsRoot(), workspaceId, name] as const,

  runsRoot: () => [...loopsKeys.all, "runs"] as const,
  // All runs-list filter permutations for one workspace — the invalidation target so a
  // run event/mutation refreshes only its own workspace's lists, never every workspace's.
  runsByWorkspace: (workspaceId: string) => [...loopsKeys.runsRoot(), workspaceId] as const,
  runs: (workspaceId: string, filters: LoopRunsFilter = {}) =>
    [
      ...loopsKeys.runsRoot(),
      workspaceId,
      normalizeText(filters.loop),
      normalizeText(filters.status),
      normalizeText(filters.origin),
      normalizeText(filters.origin_session),
      normalizeBoolean(filters.live),
      normalizeNumber(filters.limit),
    ] as const,

  runDetails: () => [...loopsKeys.all, "run-detail"] as const,
  runDetail: (workspaceId: string, runId: string) =>
    [...loopsKeys.runDetails(), workspaceId, runId] as const,

  // Run read layer (ADR-005). Three projections of one source, so they share a
  // root and invalidate together when a node verb lands or the stream reconnects.
  runReadsRoot: () => [...loopsKeys.all, "run-reads"] as const,
  runReads: (workspaceId: string, runId: string) =>
    [...loopsKeys.runReadsRoot(), workspaceId, runId] as const,
  runBriefing: (workspaceId: string, runId: string) =>
    [...loopsKeys.runReads(workspaceId, runId), "briefing"] as const,
  // `state` and `generation` are part of the key: a filtered roster is a
  // different population with its own cursor, never one list filtered on the client.
  runRoster: (workspaceId: string, runId: string, filters: LoopRosterStableFilter = {}) =>
    [
      ...loopsKeys.runReads(workspaceId, runId),
      "roster",
      normalizeText(filters.state),
      normalizeNumber(filters.generation),
      normalizeNumber(filters.limit),
    ] as const,
  // `view` is part of the key for the same reason: `notable` and `all` are two
  // histories with independently fenced cursors, not one list filtered down.
  runTimeline: (workspaceId: string, runId: string, filters: LoopTimelineStableFilter = {}) =>
    [
      ...loopsKeys.runReads(workspaceId, runId),
      "timeline",
      normalizeText(filters.view),
      normalizeNumber(filters.limit),
      normalizeNumber(filters.after_sequence),
    ] as const,

  // Workspace-scoped node inventory (`GET /loop-nodes?state=…`). `state` is a
  // required query param, so it is part of every key — the four inventory views
  // are four independent caches, never one list filtered client-side.
  nodeInventoryRoot: () => [...loopsKeys.all, "node-inventory"] as const,
  nodeInventoryByWorkspace: (workspaceId: string) =>
    [...loopsKeys.nodeInventoryRoot(), workspaceId] as const,
  nodeInventory: (workspaceId: string, filters: LoopNodeInventoryStableFilter) =>
    [
      ...loopsKeys.nodeInventoryByWorkspace(workspaceId),
      normalizeText(filters.state),
      normalizeText(filters.loop),
      normalizeText(filters.run_id),
      normalizeNumber(filters.limit),
    ] as const,
  // Existence probe (`limit: 1`) — a distinct key so the inventory infinite
  // query never shares a cache entry with the attention-bell presence check.
  nodeExists: (workspaceId: string, state: "waiting" | "attention") =>
    [...loopsKeys.all, "node-exists", workspaceId, state] as const,

  requestsRoot: () => [...loopsKeys.all, "requests"] as const,

  requestsByWorkspace: (workspaceId: string) => [...loopsKeys.requestsRoot(), workspaceId] as const,
  requestAttention: (workspaceId: string) =>
    [...loopsKeys.requestsByWorkspace(workspaceId), "attention"] as const,
  requests: (workspaceId: string, filters: LoopRequestStableFilter = {}) =>
    [
      ...loopsKeys.requestsByWorkspace(workspaceId),
      normalizeText(filters.state),
      normalizeText(filters.run_id),
      normalizeNumber(filters.limit),
    ] as const,

  requestDetail: (
    workspaceId: string,
    runId: string,
    generation: number,
    nodeId: string,
    itemIndex?: number
  ) =>
    [
      ...loopsKeys.requestsRoot(),
      "detail",
      workspaceId,
      runId,
      generation,
      nodeId,
      normalizeNumber(itemIndex),
    ] as const,

  runDiffRoot: () => [...loopsKeys.all, "run-diff"] as const,
  runDiff: (workspaceId: string, runId: string, query: LoopDiffQuery = {}) =>
    [
      ...loopsKeys.runDiffRoot(),
      workspaceId,
      runId,
      normalizeNumber(query.generation),
      normalizeNumber(query.against_generation),
      normalizeText(query.against_run),
    ] as const,

  // SSE stream resume seed (after_sequence + Last-Event-ID intent).
  streamsRoot: () => [...loopsKeys.all, "stream"] as const,
  stream: (workspaceId: string, runId: string, filters: LoopStreamFilter = {}) =>
    [
      ...loopsKeys.streamsRoot(),
      workspaceId,
      runId,
      normalizeText(filters.after_sequence),
    ] as const,
};
