import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import { LoopsApiError } from "./loops-api-errors";
import type {
  ApproveLoopRunRequest,
  LoopRun,
  LoopRunActionResult,
  LoopRunAggregates,
  LoopRunDetail,
  LoopRunsFilter,
  LoopStreamFilter,
  RunLoopRequest,
  RunLoopResult,
} from "../types";

function normalizeOptionalText(value?: string | null): string | undefined {
  if (typeof value !== "string") return undefined;
  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

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
  options: { dry?: boolean } = {},
  signal?: AbortSignal
): Promise<RunLoopResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/loops/{name}/run",
    {
      params: {
        path: { workspace_id: workspaceId, name },
        query: options.dry ? { dry: true } : {},
      },
      body,
      signal,
    }
  );

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new LoopsApiError(`Loop not found: ${name}`, 404);
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

export async function listLoopRuns(
  workspaceId: string,
  filters: LoopRunsFilter = {},
  signal?: AbortSignal
): Promise<{ runs: LoopRun[]; aggregates: LoopRunAggregates }> {
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

  const payload = requireResponseData(data, response, "Failed to fetch loop runs");
  return { runs: payload.runs, aggregates: payload.aggregates };
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
    return new LoopsApiError(
      defaultApiErrorMessage(`Cannot ${action} loop run "${runId}"`, response, error),
      response.status
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

export async function stopLoopRun(
  workspaceId: string,
  runId: string,
  signal?: AbortSignal
): Promise<LoopRunActionResult> {
  const { data, error, response } = await apiClient.POST(
    "/api/workspaces/{workspace_id}/loop-runs/{run_id}/stop",
    {
      params: { path: { workspace_id: workspaceId, run_id: runId } },
      body: {},
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    throw loopRunControlError("stop", runId, response, error);
  }
  return requireResponseData(data, response, `Failed to stop loop run "${runId}"`);
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
