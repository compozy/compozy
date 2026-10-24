import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import { LoopsApiError, normalizeOptionalText } from "./loops-api-errors";
import type {
  LoopAggregateBreadth,
  LoopCatalogBatch,
  LoopCatalogStableFilter,
  LoopRequestFilter,
  LoopRequestListResult,
  LoopRunListResult,
  LoopRunsFilter,
} from "../types";

/**
 * All projects definitions: one bounded batch (no cursor) of User rows and
 * each project's rows for one concrete profile, with per-project `errors[]`.
 */
export async function listLoopsAggregate(
  breadth: LoopAggregateBreadth,
  filters: LoopCatalogStableFilter = {},
  signal?: AbortSignal
): Promise<LoopCatalogBatch> {
  const { data, error, response } = await apiClient.GET("/api/loops", {
    params: {
      query: {
        ...breadth,
        q: normalizeOptionalText(filters.q),
        kind: filters.kind,
        category: normalizeOptionalText(filters.category),
        status: filters.status,
        sort: filters.sort,
        limit: filters.limit,
        profile: normalizeOptionalText(filters.profile),
      },
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new LoopsApiError(
      defaultApiErrorMessage("Failed to fetch loops", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch loops");
}

/** All projects runs, in recency order (the operational rank is workspace-only). */
export async function listLoopRunsAggregate(
  breadth: LoopAggregateBreadth,
  filters: LoopRunsFilter = {},
  signal?: AbortSignal
): Promise<LoopRunListResult> {
  const { data, error, response } = await apiClient.GET("/api/loop-runs", {
    params: {
      query: {
        ...breadth,
        loop: normalizeOptionalText(filters.loop),
        status: normalizeOptionalText(filters.status),
        origin: normalizeOptionalText(filters.origin),
        origin_session: normalizeOptionalText(filters.origin_session),
        live: filters.live,
        limit: filters.limit,
        cursor: normalizeOptionalText(filters.cursor),
        profile: normalizeOptionalText(filters.profile),
        all_profiles: filters.all_profiles,
      },
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new LoopsApiError(
      defaultApiErrorMessage("Failed to fetch loop runs", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch loop runs");
}

/** All projects requests, in the shared pending order; each item names its `workspace_id`. */
export async function listLoopRequestsAggregate(
  breadth: LoopAggregateBreadth,
  filters: LoopRequestFilter = {},
  signal?: AbortSignal
): Promise<LoopRequestListResult> {
  const { data, error, response } = await apiClient.GET("/api/loop-requests", {
    params: { query: { ...breadth, ...filters } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new LoopsApiError(
      defaultApiErrorMessage("Failed to fetch loop requests", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch loop requests");
}
