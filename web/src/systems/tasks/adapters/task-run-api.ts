import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  AttachTaskRunSessionRequest,
  CancelTaskRunRequest,
  CompleteTaskRunRequest,
  FailTaskRunRequest,
  ForceFailTaskRunRequest,
  ForceReleaseTaskRunRequest,
  RecoverTaskRunRequest,
  RecoverTaskRunResult,
  RetryTaskRunRequest,
  RetryTaskRunResult,
  StartTaskRunRequest,
  TaskRun,
  TaskRunDetailView,
  TaskRunInspectView,
} from "../types";
import { TasksApiError } from "./tasks-api-errors";

export async function getTaskRun(id: string, signal?: AbortSignal): Promise<TaskRunDetailView> {
  const { data, error, response } = await apiClient.GET("/api/task-runs/{id}", {
    params: { path: { id } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task run not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to fetch task run "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch task run "${id}"`).run;
}

export async function inspectRun(id: string, signal?: AbortSignal): Promise<TaskRunInspectView> {
  const { data, error, response } = await apiClient.GET("/api/runs/{id}/inspect", {
    params: { path: { id } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task run not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to inspect task run "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to inspect task run "${id}"`).inspect;
}

export async function attachTaskRunSession(
  id: string,
  body: AttachTaskRunSessionRequest,
  signal?: AbortSignal
): Promise<TaskRun> {
  return mutateRun("/api/task-runs/{id}/attach-session", id, body, "attach session to", signal);
}

export async function cancelTaskRun(
  id: string,
  body: CancelTaskRunRequest = {},
  signal?: AbortSignal
): Promise<TaskRun> {
  return mutateRun("/api/task-runs/{id}/cancel", id, body, "cancel", signal);
}

export async function startTaskRun(
  id: string,
  body: StartTaskRunRequest = {},
  signal?: AbortSignal
): Promise<TaskRun> {
  return mutateRun("/api/task-runs/{id}/start", id, body, "start", signal);
}

export async function completeTaskRun(
  id: string,
  body: CompleteTaskRunRequest = {},
  signal?: AbortSignal
): Promise<TaskRun> {
  return mutateRun("/api/task-runs/{id}/complete", id, body, "complete", signal);
}

export async function failTaskRun(
  id: string,
  body: FailTaskRunRequest,
  signal?: AbortSignal
): Promise<TaskRun> {
  return mutateRun("/api/task-runs/{id}/fail", id, body, "fail", signal);
}

export async function forceReleaseTaskRun(
  id: string,
  body: ForceReleaseTaskRunRequest = {},
  signal?: AbortSignal
): Promise<TaskRun> {
  return mutateRun("/api/runs/{id}/release", id, body, "release", signal);
}

export async function forceFailTaskRun(
  id: string,
  body: ForceFailTaskRunRequest,
  signal?: AbortSignal
): Promise<TaskRun> {
  return mutateRun("/api/runs/{id}/fail", id, body, "fail", signal);
}

export async function recoverTaskRun(
  id: string,
  body: RecoverTaskRunRequest = {},
  signal?: AbortSignal
): Promise<RecoverTaskRunResult> {
  const { data, error, response } = await apiClient.POST("/api/runs/{id}/recover", {
    params: { path: { id } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task run not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to recover task run "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to recover task run "${id}"`);
}

export async function retryTaskRun(
  id: string,
  body: RetryTaskRunRequest = {},
  signal?: AbortSignal
): Promise<RetryTaskRunResult> {
  const { data, error, response } = await apiClient.POST("/api/runs/{id}/retry", {
    params: { path: { id } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task run not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to retry task run "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to retry task run "${id}"`);
}

type TaskRunMutationPath =
  | "/api/task-runs/{id}/attach-session"
  | "/api/task-runs/{id}/cancel"
  | "/api/task-runs/{id}/start"
  | "/api/task-runs/{id}/complete"
  | "/api/task-runs/{id}/fail"
  | "/api/runs/{id}/release"
  | "/api/runs/{id}/fail";

type TaskRunMutationBody =
  | AttachTaskRunSessionRequest
  | CancelTaskRunRequest
  | StartTaskRunRequest
  | CompleteTaskRunRequest
  | FailTaskRunRequest
  | ForceReleaseTaskRunRequest
  | ForceFailTaskRunRequest;

async function mutateRun(
  path: TaskRunMutationPath,
  id: string,
  body: TaskRunMutationBody,
  action: string,
  signal?: AbortSignal
): Promise<TaskRun> {
  const { data, error, response } = await apiClient.POST(path, {
    params: { path: { id } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task run not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to ${action} task run "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to ${action} task run "${id}"`).run;
}
