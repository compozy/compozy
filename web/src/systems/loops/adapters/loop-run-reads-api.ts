import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import { LoopReadError, LoopsApiError, reasonEnvelope } from "./loops-api-errors";
import type {
  LoopBriefing,
  LoopRosterFilter,
  LoopRunRosterPage,
  LoopTimelineFilter,
  LoopTimelinePage,
} from "../types";

/**
 * The run read layer: three computed projections over one source (ADR-005).
 *
 * `briefing` is the page's verdict — served, never re-derived here (Safety
 * Invariant 12). `nodes` is the complete node × round roster, including healthy
 * nodes the lifecycle projection deliberately skips. `timeline` is the durable
 * story: its first page is the newest window and carries `head_seq`, which is
 * the fence the live SSE stream resumes from.
 */

function normalizeOptionalText(value?: string | null): string | undefined {
  if (typeof value !== "string") return undefined;
  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

function readError(fallback: string, response: Response, error: unknown): LoopsApiError {
  if (response.status === 404) {
    return new LoopsApiError("Loop run not found", 404);
  }
  const { code, details } = reasonEnvelope(error);
  if (code !== "") {
    return new LoopReadError(
      defaultApiErrorMessage(fallback, response, error),
      response.status,
      code,
      details
    );
  }
  return new LoopsApiError(defaultApiErrorMessage(fallback, response, error), response.status);
}

export async function getLoopRunBriefing(
  workspaceId: string,
  runId: string,
  signal?: AbortSignal
): Promise<LoopBriefing> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/briefing",
    {
      params: { path: { workspace_id: workspaceId, run_id: runId } },
      signal,
    }
  );

  if (apiRequestFailed(response, error)) {
    throw readError("Failed to read this run's status", response, error);
  }

  return requireResponseData(data, response, "Failed to read this run's status");
}

export async function getLoopRunRoster(
  workspaceId: string,
  runId: string,
  filters: LoopRosterFilter = {},
  signal?: AbortSignal
): Promise<LoopRunRosterPage> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/nodes",
    {
      params: {
        path: { workspace_id: workspaceId, run_id: runId },
        query: {
          // `pending` is an output state, never a filter value — the allowlist
          // the daemon accepts lives in `LOOP_ROSTER_STATE_FILTERS`.
          state: normalizeOptionalText(filters.state),
          generation: filters.generation,
          cursor: normalizeOptionalText(filters.cursor),
          limit: filters.limit,
        },
      },
      signal,
    }
  );

  if (apiRequestFailed(response, error)) {
    throw readError("Failed to read this run's steps", response, error);
  }

  return requireResponseData(data, response, "Failed to read this run's steps");
}

export async function getLoopRunTimeline(
  workspaceId: string,
  runId: string,
  filters: LoopTimelineFilter = {},
  signal?: AbortSignal
): Promise<LoopTimelinePage> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/timeline",
    {
      params: {
        path: { workspace_id: workspaceId, run_id: runId },
        query: {
          view: normalizeOptionalText(filters.view),
          // Backward paging is the opaque cursor only; it binds
          // {run, view, fixed head, before} so appends never shift a page set.
          cursor: normalizeOptionalText(filters.cursor),
          limit: filters.limit,
          after_sequence: filters.after_sequence,
        },
      },
      signal,
    }
  );

  if (apiRequestFailed(response, error)) {
    throw readError("Failed to read this run's story", response, error);
  }

  return requireResponseData(data, response, "Failed to read this run's story");
}
