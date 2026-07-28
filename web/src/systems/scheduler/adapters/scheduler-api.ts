import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  SchedulerBacklog,
  SchedulerBacklogQuery,
  SchedulerDrainRequest,
  SchedulerDrainResult,
  SchedulerPauseRequest,
  SchedulerResumeRequest,
  SchedulerStatus,
} from "../types";

export class SchedulerApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "SchedulerApiError";
  }
}

function normalizeOptionalText(value?: string | null): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }

  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

function normalizeBacklogQuery(query: SchedulerBacklogQuery = {}): SchedulerBacklogQuery {
  return {
    include_paused: query.include_paused,
    limit: query.limit,
    scope: query.scope,
    workspace: normalizeOptionalText(query.workspace),
  };
}

export async function getScheduler(signal?: AbortSignal): Promise<SchedulerStatus> {
  const { data, error, response } = await apiClient.GET("/api/scheduler", { signal });

  if (apiRequestFailed(response, error)) {
    throw new SchedulerApiError(
      defaultApiErrorMessage("Failed to fetch scheduler status", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to fetch scheduler status").scheduler;
}

export async function pauseScheduler(
  body: SchedulerPauseRequest = {},
  signal?: AbortSignal
): Promise<SchedulerStatus> {
  const { data, error, response } = await apiClient.POST("/api/scheduler/pause", {
    body,
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new SchedulerApiError(
      defaultApiErrorMessage("Failed to pause scheduler", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to pause scheduler").scheduler;
}

export async function resumeScheduler(
  body: SchedulerResumeRequest = {},
  signal?: AbortSignal
): Promise<SchedulerStatus> {
  const { data, error, response } = await apiClient.POST("/api/scheduler/resume", {
    body,
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new SchedulerApiError(
      defaultApiErrorMessage("Failed to resume scheduler", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to resume scheduler").scheduler;
}

export async function drainScheduler(
  body: SchedulerDrainRequest = {},
  signal?: AbortSignal
): Promise<SchedulerDrainResult> {
  const { data, error, response } = await apiClient.POST("/api/scheduler/drain", {
    body,
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new SchedulerApiError(
      defaultApiErrorMessage("Failed to drain scheduler", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to drain scheduler");
}

export async function getSchedulerBacklog(
  query: SchedulerBacklogQuery = {},
  signal?: AbortSignal
): Promise<SchedulerBacklog> {
  const { data, error, response } = await apiClient.GET("/api/scheduler/backlog", {
    params: { query: normalizeBacklogQuery(query) },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new SchedulerApiError(
      defaultApiErrorMessage("Failed to fetch scheduler backlog", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to fetch scheduler backlog").backlog;
}
