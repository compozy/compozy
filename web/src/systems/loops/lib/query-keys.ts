import { normalizeLoopCatalogFilter } from "./loops-list-query";
import type {
  GoalTurnFilter,
  LoopCatalogStableFilter,
  LoopRunsFilter,
  LoopStreamFilter,
} from "../types";

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
