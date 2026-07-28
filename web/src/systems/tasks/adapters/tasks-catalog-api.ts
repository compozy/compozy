import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import { normalizeTaskListFilter } from "../lib/task-list-query";
import type {
  AddTaskDependencyRequest,
  CancelTaskRequest,
  CreateChildTaskRequest,
  CreateTaskRequest,
  PauseTaskRequest,
  RecoverTaskRequest,
  ResumeTaskRequest,
  TaskDetailView,
  TaskInspectView,
  TaskListFilter,
  TaskListPage,
  TaskRecord,
  UpdateTaskRequest,
} from "../types";
import { TasksApiError } from "./tasks-api-errors";

export async function listTasks(
  filters: TaskListFilter = {},
  signal?: AbortSignal
): Promise<TaskListPage> {
  const { data, error, response } = await apiClient.GET("/api/tasks", {
    params: { query: normalizeTaskListFilter(filters) },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new TasksApiError(
      defaultApiErrorMessage("Failed to fetch tasks", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to fetch tasks");
}

export async function getTask(id: string, signal?: AbortSignal): Promise<TaskDetailView> {
  const { data, error, response } = await apiClient.GET("/api/tasks/{id}", {
    params: { path: { id } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new TasksApiError(`Task not found: ${id}`, 404);
    }
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to fetch task "${id}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to fetch task "${id}"`).task;
}

export async function inspectTask(id: string, signal?: AbortSignal): Promise<TaskInspectView> {
  const { data, error, response } = await apiClient.GET("/api/tasks/{id}/inspect", {
    params: { path: { id } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new TasksApiError(`Task not found: ${id}`, 404);
    }
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to inspect task "${id}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to inspect task "${id}"`).inspect;
}

export async function deleteTask(id: string, signal?: AbortSignal): Promise<void> {
  const { error, response } = await apiClient.DELETE("/api/tasks/{id}", {
    params: { path: { id } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new TasksApiError(`Task not found: ${id}`, 404);
    }
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to delete task "${id}"`, response, error),
      response.status
    );
  }
}

export async function createTask(
  body: CreateTaskRequest,
  signal?: AbortSignal
): Promise<TaskRecord> {
  const { data, error, response } = await apiClient.POST("/api/tasks", { body, signal });
  if (apiRequestFailed(response, error)) {
    throw new TasksApiError(
      defaultApiErrorMessage("Failed to create task", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to create task").task;
}

export async function updateTask(
  id: string,
  body: UpdateTaskRequest,
  signal?: AbortSignal
): Promise<TaskRecord> {
  const { data, error, response } = await apiClient.PATCH("/api/tasks/{id}", {
    params: { path: { id } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new TasksApiError(`Task not found: ${id}`, 404);
    }
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to update task "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to update task "${id}"`).task;
}

export async function publishTask(id: string, signal?: AbortSignal): Promise<TaskRecord> {
  const { data, error, response } = await apiClient.POST("/api/tasks/{id}/publish", {
    params: { path: { id } },
    body: {},
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to publish task "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to publish task "${id}"`).task;
}

export async function cancelTask(
  id: string,
  body: CancelTaskRequest = {},
  signal?: AbortSignal
): Promise<TaskRecord> {
  const { data, error, response } = await apiClient.POST("/api/tasks/{id}/cancel", {
    params: { path: { id } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to cancel task "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to cancel task "${id}"`).task;
}

export async function pauseTask(
  id: string,
  body: PauseTaskRequest,
  signal?: AbortSignal
): Promise<TaskRecord> {
  const { data, error, response } = await apiClient.POST("/api/tasks/{id}/pause", {
    params: { path: { id } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to pause task "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to pause task "${id}"`).task;
}

export async function resumeTask(
  id: string,
  body: ResumeTaskRequest = {},
  signal?: AbortSignal
): Promise<TaskRecord> {
  const { data, error, response } = await apiClient.POST("/api/tasks/{id}/resume", {
    params: { path: { id } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to resume task "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to resume task "${id}"`).task;
}

export async function recoverTask(
  id: string,
  body: RecoverTaskRequest = {},
  signal?: AbortSignal
): Promise<TaskRecord> {
  const { data, error, response } = await apiClient.POST("/api/tasks/{id}/recover", {
    params: { path: { id } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${id}`, 404);
    if (response.status === 409) {
      throw new TasksApiError(
        defaultApiErrorMessage(`Task "${id}" is not in needs_attention`, response, error),
        409
      );
    }
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to recover task "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to recover task "${id}"`).task;
}

export async function approveTask(id: string, signal?: AbortSignal): Promise<TaskRecord> {
  const { data, error, response } = await apiClient.POST("/api/tasks/{id}/approve", {
    params: { path: { id } },
    body: {},
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to approve task "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to approve task "${id}"`).task;
}

export async function rejectTask(id: string, signal?: AbortSignal): Promise<TaskRecord> {
  const { data, error, response } = await apiClient.POST("/api/tasks/{id}/reject", {
    params: { path: { id } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to reject task "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to reject task "${id}"`).task;
}

export async function clearTaskBlock(
  id: string,
  blockId: string,
  note?: string,
  signal?: AbortSignal
): Promise<void> {
  const { error, response } = await apiClient.POST("/api/tasks/{id}/blocks/{block_id}/clear", {
    params: { path: { id, block_id: blockId } },
    body: note ? { note } : {},
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new TasksApiError(`Task "${id}" or block "${blockId}" was not found`, 404);
    }
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to clear block "${blockId}" on task "${id}"`, response, error),
      response.status
    );
  }
}

export async function createChildTask(
  parentId: string,
  body: CreateChildTaskRequest,
  signal?: AbortSignal
): Promise<TaskRecord> {
  const { data, error, response } = await apiClient.POST("/api/tasks/{id}/children", {
    params: { path: { id: parentId } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${parentId}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to create child task for "${parentId}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to create child task for "${parentId}"`).task;
}

export async function addTaskDependency(
  id: string,
  body: AddTaskDependencyRequest,
  signal?: AbortSignal
): Promise<TaskDetailView> {
  const { data, error, response } = await apiClient.POST("/api/tasks/{id}/dependencies", {
    params: { path: { id } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to add dependency to task "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to add dependency to task "${id}"`).task;
}

export async function removeTaskDependency(
  id: string,
  dependsOnId: string,
  signal?: AbortSignal
): Promise<TaskDetailView> {
  const { data, error, response } = await apiClient.DELETE(
    "/api/tasks/{id}/dependencies/{depends_on_id}",
    {
      params: { path: { id, depends_on_id: dependsOnId } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${id}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to remove dependency from task "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to remove dependency from task "${id}"`).task;
}
