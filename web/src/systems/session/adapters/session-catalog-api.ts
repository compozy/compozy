import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import { normalizeSessionListFilters } from "../lib/session-list-query";
import type { SessionCatalogPageResponse, SessionsQuery, SessionsResponse } from "../types";
import { throwSessionRequestError } from "./session-api-errors";

function normalizeSessionsQuery(query: SessionsQuery): SessionsQuery | undefined {
  const { cursor, ...filters } = query;
  const normalizedCursor = cursor?.trim();
  const normalized: SessionsQuery = {
    ...normalizeSessionListFilters(filters),
    ...(normalizedCursor ? { cursor: normalizedCursor } : {}),
  };

  return Object.keys(normalized).length > 0 ? normalized : undefined;
}

export async function fetchSessionCatalogPage(
  query: SessionsQuery = {},
  signal?: AbortSignal
): Promise<SessionCatalogPageResponse> {
  const { data, error, response } = await apiClient.GET("/api/sessions", {
    params: { query: normalizeSessionsQuery(query) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to fetch sessions");
  }
  return requireResponseData(data, response, "Failed to fetch sessions");
}

/** Exact counted catalog for consumers that present population totals. */
export async function fetchSessions(
  query: SessionsQuery = {},
  signal?: AbortSignal
): Promise<SessionsResponse> {
  const response = await fetchSessionCatalogPage(query, signal);
  if (response.page.total == null) throw new Error("Session catalog count was not returned");
  return { ...response, page: { ...response.page, total: response.page.total } };
}

export async function fetchSessionFacets(query: SessionsQuery = {}, signal?: AbortSignal) {
  const { data, error, response } = await apiClient.GET("/api/sessions/facets", {
    params: { query: normalizeSessionsQuery(query) },
    signal,
  });
  if (apiRequestFailed(response, error))
    throwSessionRequestError(response, error, "Failed to fetch session counts");
  return requireResponseData(data, response, "Failed to fetch session counts");
}
