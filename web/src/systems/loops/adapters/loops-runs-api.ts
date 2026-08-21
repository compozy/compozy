import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import {
  LoopInputValidationError,
  LoopLifecycleConflictError,
  LoopsApiError,
  inputValidationPayload,
  normalizeOptionalText,
  reasonEnvelope,
} from "./loops-api-errors";
import type {
  ApproveLoopRunRequest,
  LoopRunActionResult,
  LoopRunMutationResult,
  LoopRunDetail,
  LoopRunListResult,
  LoopRunsFilter,
  LoopStreamFilter,
  RunLoopRequest,
  RunLoopResult,
} from "../types";

export function buildLoopStreamUrl(
  workspaceId: string,
  runId: string,
  filters: LoopStreamFilter = {}
): string {
  const trimmedWorkspace = workspaceId.trim();
  if (trimmedWorkspace === "") {
    throw new LoopsApiError("workspace id is required to build stream url", 400);
  }
  const trimmedRun = runId.trim();
  if (trimmedRun === "") {
    throw new LoopsApiError("loop run id is required to build stream url", 400);
  }
  const path = `/api/workspaces/${encodeURIComponent(trimmedWorkspace)}/loop-runs/${encodeURIComponent(
    trimmedRun
  )}/events`;
  return filters.after_sequence === undefined
    ? path
    : `${path}?after_sequence=${encodeURIComponent(String(filters.after_sequence))}`;
}

export async function runLoop(
  workspaceId: string,
  name: string,
  body: RunLoopRequest,
  options: { dry?: boolean; profile?: string } = {},
  signal?: AbortSignal
): Promise<RunLoopResult> {
  const profile = options.profile?.trim();
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/loops/{name}/run",
    {
      params: {
        path: { workspace_id: workspaceId, name },
        // A run is owned work: the profile it is started as decides who the run
        // belongs to, and an omitted selector would hand it to `default`.
        query: {
          ...(options.dry ? { dry: true } : {}),
          ...(profile ? { profile } : {}),
        },
      },
      body,
      signal,
    }
  );

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new LoopsApiError(`Loop not found: ${name}`, 404);
    if (response.status === 422) {
      const validation = inputValidationPayload(error);
      if (validation) throw new LoopInputValidationError(validation);
    }
    if (response.status === 409) {
      throw new LoopsApiError(
        defaultApiErrorMessage(`Loop "${name}" is already running`, response, error),
        409
      );
    }
    throw new LoopsApiError(
      defaultApiErrorMessage(`Failed to run loop "${name}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to run loop "${name}"`);
}

/**
 * The workspace run list, returned as the server sent it.
 *
 * The envelope keeps its `next_cursor`: a roster that only ever renders the
 * first page still has to be able to say that a second one exists, and a view
 * model cannot recover a continuation token the adapter already dropped. The
 * flattening happens where the view reads it, not here (R09).
 */
export async function listLoopRuns(
  workspaceId: string,
  filters: LoopRunsFilter = {},
  signal?: AbortSignal
): Promise<LoopRunListResult> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/loop-runs",
    {
      params: {
        path: { workspace_id: workspaceId },
        query: {
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
    }
  );

  if (apiRequestFailed(response, error)) {
    throw new LoopsApiError(
      defaultApiErrorMessage("Failed to fetch loop runs", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to fetch loop runs");
}

export async function getLoopRun(
  workspaceId: string,
  runId: string,
  signal?: AbortSignal
): Promise<LoopRunDetail> {
  const { data, error, response } = await apiClient.GET(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}",
    {
      params: { path: { workspace_id: workspaceId, run_id: runId } },
      signal,
    }
  );

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new LoopsApiError(`Loop run not found: ${runId}`, 404);
    throw new LoopsApiError(
      defaultApiErrorMessage(`Failed to fetch loop run "${runId}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to fetch loop run "${runId}"`);
}

function loopRunControlError(
  action: string,
  runId: string,
  response: Response,
  error: unknown
): LoopsApiError {
  if (response.status === 404) return new LoopsApiError(`Loop run not found: ${runId}`, 404);
  if (response.status === 409 || response.status === 422) {
    const { code, details } = reasonEnvelope(error);
    return new LoopLifecycleConflictError(
      defaultApiErrorMessage(`Cannot ${action} loop run "${runId}"`, response, error),
      response.status,
      code,
      details
    );
  }
  return new LoopsApiError(
    defaultApiErrorMessage(`Failed to ${action} loop run "${runId}"`, response, error),
    response.status
  );
}

export async function pauseLoopRun(
  workspaceId: string,
  runId: string,
  signal?: AbortSignal
): Promise<LoopRunActionResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/pause",
    {
      params: { path: { workspace_id: workspaceId, run_id: runId } },
      body: {},
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw loopRunControlError("pause", runId, response, error);
  }
  return requireResponseData(data, response, `Failed to pause loop run "${runId}"`);
}

export async function resumeLoopRun(
  workspaceId: string,
  runId: string,
  signal?: AbortSignal
): Promise<LoopRunActionResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/resume",
    {
      params: { path: { workspace_id: workspaceId, run_id: runId } },
      body: {},
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw loopRunControlError("resume", runId, response, error);
  }
  return requireResponseData(data, response, `Failed to resume loop run "${runId}"`);
}

export async function cancelLoopRun(
  workspaceId: string,
  runId: string,
  signal?: AbortSignal
): Promise<LoopRunMutationResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/cancel",
    {
      params: { path: { workspace_id: workspaceId, run_id: runId } },
      body: {},
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw loopRunControlError("cancel", runId, response, error);
  }
  return requireResponseData(data, response, `Failed to cancel loop run "${runId}"`);
}

/**
 * Kill stops a live run immediately: in-flight work is interrupted mid-step and
 * its reactions are skipped. Cancel winds the same run down cooperatively. Both
 * land on the `canceled` terminal, which is why they share a response shape.
 */
export async function killLoopRun(
  workspaceId: string,
  runId: string,
  signal?: AbortSignal
): Promise<LoopRunMutationResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/kill",
    {
      params: { path: { workspace_id: workspaceId, run_id: runId } },
      body: {},
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw loopRunControlError("kill", runId, response, error);
  }
  return requireResponseData(data, response, `Failed to kill loop run "${runId}"`);
}

export async function approveLoopRun(
  workspaceId: string,
  runId: string,
  body: ApproveLoopRunRequest,
  signal?: AbortSignal
): Promise<LoopRunActionResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/approve",
    {
      params: { path: { workspace_id: workspaceId, run_id: runId } },
      body,
      signal,
    }
  );

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new LoopsApiError(`Loop run not found: ${runId}`, 404);
    if (response.status === 409 || response.status === 422) {
      throw new LoopsApiError(
        defaultApiErrorMessage(`Cannot record gate decision for run "${runId}"`, response, error),
        response.status
      );
    }
    throw new LoopsApiError(
      defaultApiErrorMessage(`Failed to approve loop run "${runId}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to approve loop run "${runId}"`);
}
