import { normalizeLoopCatalogFilter } from "./loops-list-query";
import type {
  GoalTurnFilter,
  LoopCatalogStableFilter,
  LoopDiffQuery,
  LoopNodeInventoryFilter,
  LoopRequestStableFilter,
  LoopRunsFilter,
  LoopStreamFilter,
} from "../types";

/** Inventory filter minus the continuation cursor, which lives in `pageParam`. */
export type LoopNodeInventoryStableFilter = Omit<LoopNodeInventoryFilter, "cursor">;

function normalizeText(value?: string | null): string {
  // Trim to match the adapter's `normalizeOptionalText`, so a whitespace-padded
  // filter never keys a duplicate cache entry for an identical request.
  return typeof value === "string" ? value.trim() : "";
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
  runRequestCounts: (workspaceId: string) =>
    [...loopsKeys.requestsByWorkspace(workspaceId), "run-counts"] as const,
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

  goalTurnsRoot: () => [...loopsKeys.all, "goal-turns"] as const,
  goalTurns: (
    workspaceId: string,
    runId: string,
    filters: Pick<GoalTurnFilter, "node" | "item" | "limit"> = {}
  ) =>
    [
      ...loopsKeys.goalTurnsRoot(),
      workspaceId,
      runId,
      normalizeText(filters.node),
      normalizeNumber(filters.item),
      normalizeNumber(filters.limit),
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
