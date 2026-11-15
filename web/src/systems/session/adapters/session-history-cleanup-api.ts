import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import type { components } from "@compozy/client";
import { throwSessionRequestError } from "./session-api-errors";

export type SessionHistoryCleanupRequest = components["schemas"]["SessionHistoryCleanupRequest"];
export type SessionHistoryCleanupPlan = components["schemas"]["SessionHistoryCleanupPlan"];
export type SessionHistoryCleanupRun = components["schemas"]["SessionHistoryCleanupRun"];

const CLEANUP_PATH = "/api/sessions/history-cleanup";

/**
 * Dry-runs a cleanup of one profile's archived sessions: what the run would delete, by kind,
 * per workspace, and what each check alone would delete. Nothing is deleted.
 */
export async function previewSessionHistoryCleanup(
  profile: string,
  body: SessionHistoryCleanupRequest,
  signal?: AbortSignal
): Promise<SessionHistoryCleanupPlan> {
  const { data, error, response } = await apiClient.POST(CLEANUP_PATH, {
    params: { query: { profile } },
    body: { ...body, dry_run: true },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to count archived sessions");
  }
  return requireResponseData(data, response, "Failed to count archived sessions").plan;
}

/** Starts the cleanup in the daemon's background; the run reports progress on the catalog stream. */
export async function startSessionHistoryCleanup(
  profile: string,
  body: SessionHistoryCleanupRequest,
  signal?: AbortSignal
): Promise<SessionHistoryCleanupRun> {
  const { data, error, response } = await apiClient.POST(CLEANUP_PATH, {
    params: { query: { profile } },
    body: { ...body, dry_run: false },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to start cleaning up history");
  }
  // The route answers a dry run with 200 and a started run with 202; only the latter has a run.
  const started = requireResponseData(data, response, "Failed to start cleaning up history");
  if (!("run" in started)) {
    throw new Error("Failed to start cleaning up history: no run was started");
  }
  return started.run;
}

/** The profile's running or most recent cleanup; `null` when none ran since the daemon started. */
export async function fetchSessionHistoryCleanupRun(
  profile: string,
  signal?: AbortSignal
): Promise<SessionHistoryCleanupRun | null> {
  const { data, error, response } = await apiClient.GET(CLEANUP_PATH, {
    params: { query: { profile } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to read the history cleanup");
  }
  return requireResponseData(data, response, "Failed to read the history cleanup").run ?? null;
}

/** Stops the running cleanup after the deletes in flight; what it deleted stays deleted. */
export async function stopSessionHistoryCleanup(
  profile: string,
  signal?: AbortSignal
): Promise<SessionHistoryCleanupRun | null> {
  const { data, error, response } = await apiClient.POST(`${CLEANUP_PATH}/stop`, {
    params: { query: { profile } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throwSessionRequestError(response, error, "Failed to stop cleaning up history");
  }
  return requireResponseData(data, response, "Failed to stop cleaning up history").run ?? null;
}
