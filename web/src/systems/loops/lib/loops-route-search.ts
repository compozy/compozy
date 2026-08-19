import type { ListingViewMode } from "@compozy/ui";

import { LOOP_RUN_STATUSES } from "@/generated/loop-enums";
import { normalizeListingSearchValue, parseListingView } from "@/lib/listing-search";
import type { LoopKindFilter, LoopStatusFilter } from "./loop-catalog";
import { isLoopNodeInventoryState } from "./loop-node-inventory-state";
import type { LoopDiffQuery, LoopNodeInventoryState, LoopRunStatus } from "../types";

export interface LoopsRouteSearch {
  q?: string;
  view?: ListingViewMode;
  kind?: Exclude<LoopKindFilter, "all">;
  category?: string;
  status?: LoopStatusFilter;
  workspace?: string;
}

export interface LoopRunsRouteSearch {
  origin?: "catalog" | "session";
  origin_session?: string;
  nodes?: LoopNodeInventoryState;
  nodes_loop?: string;
  nodes_run?: string;
  view?: ListingViewMode;
  workspace?: string;
}

export interface LoopRunDiffRouteSearch {
  generation?: number;
  against_generation?: number;
  against_run?: string;
  workspace?: string;
}

export function parseLoopKindFilter(value: unknown): Exclude<LoopKindFilter, "all"> | undefined {
  return value === "read-only" || value === "workspace" ? value : undefined;
}

export function parseLoopCategoryFilter(value: unknown): string | undefined {
  return normalizeListingSearchValue(value);
}

export function parseLoopGeneration(value: unknown): number | undefined {
  if (typeof value === "number") {
    return Number.isInteger(value) && value >= 0 ? value : undefined;
  }
  const text = normalizeListingSearchValue(value);
  if (text === undefined) {
    return undefined;
  }
  const parsed = Number(text);
  return Number.isInteger(parsed) && parsed >= 0 ? parsed : undefined;
}

export function parseLoopStatusFilter(value: unknown): LoopRunStatus | undefined {
  return typeof value === "string" && (LOOP_RUN_STATUSES as readonly string[]).includes(value)
    ? (value as LoopRunStatus)
    : undefined;
}

export function validateLoopsSearch(search: Record<string, unknown>): LoopsRouteSearch {
  return {
    category: parseLoopCategoryFilter(search.category),
    kind: parseLoopKindFilter(search.kind),
    q: normalizeListingSearchValue(search.q),
    status: parseLoopStatusFilter(search.status),
    view: parseListingView(search.view),
    workspace: normalizeListingSearchValue(search.workspace),
  };
}

export function validateLoopRunDiffSearch(search: Record<string, unknown>): LoopRunDiffRouteSearch {
  const againstRun = normalizeListingSearchValue(search.against_run);
  return {
    generation: parseLoopGeneration(search.generation),
    against_generation:
      againstRun === undefined ? parseLoopGeneration(search.against_generation) : undefined,
    against_run: againstRun,
    workspace: normalizeListingSearchValue(search.workspace),
  };
}

export function loopRunDiffQuery(
  search: LoopRunDiffRouteSearch
): NonNullable<LoopDiffQuery> | null {
  if (search.against_run !== undefined) {
    return { generation: search.generation, against_run: search.against_run };
  }
  if (search.against_generation !== undefined) {
    return { generation: search.generation, against_generation: search.against_generation };
  }
  return null;
}

export function validateLoopRunsSearch(search: Record<string, unknown>): LoopRunsRouteSearch {
  const origin =
    search.origin === "catalog" || search.origin === "session" ? search.origin : undefined;
  const originSession = normalizeListingSearchValue(search.origin_session);
  const nodes = isLoopNodeInventoryState(search.nodes) ? search.nodes : undefined;
  const nodesLoop = normalizeListingSearchValue(search.nodes_loop);
  const nodesRun = normalizeListingSearchValue(search.nodes_run);
  return {
    origin,
    origin_session: originSession,
    nodes,
    nodes_loop: nodes === undefined ? undefined : nodesLoop,
    nodes_run: nodes === undefined ? undefined : nodesRun,
    view: parseListingView(search.view),
    workspace: normalizeListingSearchValue(search.workspace),
  };
}
