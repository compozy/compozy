import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { SessionAttentionSummary } from "../types";
import { throwSessionRequestError } from "./session-api-errors";

/**
 * Exact cross-workspace attention counts. This projection — never a count over
 * a loaded row page — feeds the bell badge and the tab title, so the number
 * stays true past any page size. Operator scope: a request carrying agent
 * identity is denied by the daemon.
 */
export async function fetchSessionAttentionSummary(
  signal?: AbortSignal
): Promise<SessionAttentionSummary> {
  const { data, error, response } = await apiClient.GET("/api/sessions/attention-summary", {
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to fetch attention summary");
  }
  return requireResponseData(data, response, "Failed to fetch attention summary");
}
