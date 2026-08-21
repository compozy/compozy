import { apiClient, apiRequestFailed, defaultApiErrorMessage } from "@/lib/api-client";

import {
  LoopReadError,
  LoopsApiError,
  normalizeOptionalText,
  reasonEnvelope,
} from "./loops-api-errors";
import { LOOP_ROSTER_STATE_FILTERS, isLoopRosterStateFilter } from "./loop-roster-filters";
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

/**
 * The one allowlist, applied rather than merely cited.
 *
 * `pending` is an output state, never a filter value, so asking for it earns a
 * `400 invalid_node_state` (peer review B-007, UT-050). The daemon stays the
 * authority — its refusal carries the allowed set and reaches the UI as
 * `LoopReadError.allowedStates` — but sending a value we already know it
 * rejects spends a round trip to learn something this list already knows.
 * `LOOP_ROSTER_STATE_FILTERS` is that list, and it is the same one the MSW
 * resolvers validate against, so the two cannot drift.
 */
function rosterStateFilter(value?: string | null): string | undefined {
  const state = normalizeOptionalText(value);
  if (state === undefined) return undefined;
  if (!isLoopRosterStateFilter(state)) {
    throw new LoopReadError(
      `Unsupported roster state filter: ${state}`,
      400,
      "invalid_node_state",
      {
        allowed: LOOP_ROSTER_STATE_FILTERS.join(","),
      }
    );
  }
  return state;
}

const BRIEFING_FAILED = "Failed to read this run's status";
const ROSTER_FAILED = "Failed to read this run's steps";
const TIMELINE_FAILED = "Failed to read this run's story";

/**
 * Every read rejection the daemon can structure, kept structured.
 *
 * `respondLoopRunReadError` answers with `{error, code, details}` for all four
 * of its named refusals — including the 404, whose `details.run_id` names the
 * run that was missed. Collapsing that into a prose-only error would throw away
 * the `allowed` state list and the stale-cursor signal the story recovers from,
 * so the code path is the same for every status: parse once, keep the type.
 */
function readError(fallback: string, response: Response, error: unknown): LoopsApiError {
  const message =
    response.status === 404
      ? "Loop run not found"
      : defaultApiErrorMessage(fallback, response, error);
  const { code, details } = reasonEnvelope(error);
  if (code !== "") {
    return new LoopReadError(message, response.status, code, details);
  }
  return new LoopsApiError(message, response.status);
}

/**
 * A read that answered 2xx with no body is still a failed read, and it has to
 * arrive as a `LoopsApiError` like every other one — a bare `Error` would escape
 * the adapters' typed-error boundary and reach the hooks unclassifiable.
 */
function readData<T>(data: T | undefined, response: Response, fallback: string): T {
  if (data === undefined) {
    throw new LoopsApiError(`${fallback}: empty response (${response.status})`, response.status);
  }
  return data;
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
    throw readError(BRIEFING_FAILED, response, error);
  }

  return readData(data, response, BRIEFING_FAILED);
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
          state: rosterStateFilter(filters.state),
          generation: filters.generation,
          cursor: normalizeOptionalText(filters.cursor),
          limit: filters.limit,
        },
      },
      signal,
    }
  );

  if (apiRequestFailed(response, error)) {
    throw readError(ROSTER_FAILED, response, error);
  }

  return readData(data, response, ROSTER_FAILED);
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
    throw readError(TIMELINE_FAILED, response, error);
  }

  return readData(data, response, TIMELINE_FAILED);
}
