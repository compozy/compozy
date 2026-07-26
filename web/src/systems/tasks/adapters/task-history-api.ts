import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  EnqueueTaskRunRequest,
  FanOutTaskRunsRequest,
  FanOutTaskRunsResponse,
  TaskRun,
  TaskRunsFilter,
  TaskTimelineFilter,
  TaskTimelineItem,
  TaskTreeView,
  TaskTriageState,
} from "../types";
import { normalizeOptionalText, TasksApiError } from "./tasks-api-errors";

export async function listTaskRuns(
  id: string,
  filters: TaskRunsFilter = {},
  signal?: AbortSignal
): Promise<TaskRun[]> {
  const { data, error, response } = await apiClient.GET("/api/tasks/{id}/runs", {
    params: {
      path: { id },
      query: {
        status: filters.status,
        session_id: normalizeOptionalText(filters.session_id),
        limit: filters.limit,
      },
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to fetch runs for task "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch runs for task "${id}"`).runs;
}

export async function enqueueTaskRun(
  id: string,
  body: EnqueueTaskRunRequest = {},
  signal?: AbortSignal
): Promise<TaskRun> {
  const { data, error, response } = await apiClient.POST("/api/tasks/{id}/runs", {
    params: { path: { id } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to enqueue run for task "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to enqueue run for task "${id}"`).run;
}

export async function fanOutTaskRuns(
  id: string,
  body: FanOutTaskRunsRequest,
  signal?: AbortSignal
): Promise<FanOutTaskRunsResponse> {
  const { data, error, response } = await apiClient.POST("/api/tasks/{id}/runs/fan-out", {
    params: { path: { id } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to fan out runs for task "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fan out runs for task "${id}"`);
}

export async function getTaskTimeline(
  id: string,
  filters: TaskTimelineFilter = {},
  signal?: AbortSignal
): Promise<TaskTimelineItem[]> {
  const { data, error, response } = await apiClient.GET("/api/tasks/{id}/timeline", {
    params: {
      path: { id },
      query: { after_sequence: filters.after_sequence, limit: filters.limit },
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to fetch timeline for task "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch timeline for task "${id}"`).timeline;
}

export async function getTaskTree(id: string, signal?: AbortSignal): Promise<TaskTreeView> {
  const { data, error, response } = await apiClient.GET("/api/tasks/{id}/tree", {
    params: { path: { id } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to fetch tree for task "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch tree for task "${id}"`).tree;
}

async function mutateTriage(
  id: string,
  action: "read" | "archive" | "dismiss",
  signal?: AbortSignal
): Promise<TaskTriageState> {
  const operation =
    action === "read" ? "mark as read" : action === "archive" ? "archive" : "dismiss";
  const request =
    action === "read"
      ? apiClient.POST("/api/tasks/{id}/triage/read", { params: { path: { id } }, signal })
      : action === "archive"
        ? apiClient.POST("/api/tasks/{id}/triage/archive", { params: { path: { id } }, signal })
        : apiClient.POST("/api/tasks/{id}/triage/dismiss", { params: { path: { id } }, signal });
  const { data, error, response } = await request;
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to ${operation} task "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to ${operation} task "${id}"`).triage;
}

export function markTaskRead(id: string, signal?: AbortSignal): Promise<TaskTriageState> {
  return mutateTriage(id, "read", signal);
}

export function archiveTask(id: string, signal?: AbortSignal): Promise<TaskTriageState> {
  return mutateTriage(id, "archive", signal);
}

export function dismissTask(id: string, signal?: AbortSignal): Promise<TaskTriageState> {
  return mutateTriage(id, "dismiss", signal);
}
