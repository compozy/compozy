import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type { TaskExecutionProfile, TaskWorktreePolicySetRequest } from "../types";
import { TasksApiError } from "./tasks-api-errors";

/**
 * Sets the worktree policy alone.
 *
 * This is the only write path the web uses for that block: the PUT route
 * replaces the whole execution profile, so routing a policy change through it
 * would make an unrelated block's staleness a silent overwrite. The daemon
 * read-modify-writes and returns the full profile so callers reseed from truth.
 * The same active-run lock as PUT applies; a locked task answers 409.
 */
export async function setTaskWorktreePolicy(
  id: string,
  body: TaskWorktreePolicySetRequest,
  signal?: AbortSignal
): Promise<TaskExecutionProfile> {
  const { data, error, response } = await apiClient.PATCH(
    "/api/tasks/{id}/execution-profile/worktree",
    {
      params: { path: { id } },
      body,
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${id}`, 404);
    if (response.status === 409) {
      throw new TasksApiError(
        defaultApiErrorMessage(`Execution profile conflict for task "${id}"`, response, error),
        409
      );
    }
    throw new TasksApiError(
      defaultApiErrorMessage(`Failed to set worktree policy for task "${id}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to set worktree policy for task "${id}"`)
    .profile;
}
