import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type { AgentContextView, TaskContextBundle } from "../types";
import { TasksApiError } from "./tasks-api-errors";

export async function getAgentContext(signal?: AbortSignal): Promise<AgentContextView> {
  const { data, error, response } = await apiClient.GET("/api/agent/context", { signal });
  if (apiRequestFailed(response, error)) {
    throw new TasksApiError(
      defaultApiErrorMessage("Failed to fetch agent context", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch agent context").context;
}

export async function getTaskContextBundle(
  signal?: AbortSignal
): Promise<TaskContextBundle | null> {
  const context = await getAgentContext(signal);
  return context.task.bundle ?? null;
}
