import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import { normalizeSessionListFilters } from "../lib/session-list-query";
import type { SessionsQuery, SessionsResponse } from "../types";
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

export async function fetchSessions(
  query: SessionsQuery = {},
  signal?: AbortSignal
): Promise<SessionsResponse> {
  const { data, error, response } = await apiClient.GET("/api/sessions", {
    params: { query: normalizeSessionsQuery(query) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to fetch sessions");
  }
  return requireResponseData(data, response, "Failed to fetch sessions");
}
