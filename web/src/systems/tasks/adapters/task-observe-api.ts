import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import { normalizeTaskInboxFilter } from "../lib/task-inbox-query";
import type {
  TaskDashboardFilter,
  TaskDashboardView,
  TaskInboxFilter,
  TaskInboxView,
} from "../types";
import { normalizeOptionalText, TasksApiError } from "./tasks-api-errors";

function normalizeDashboardFilter(filters: TaskDashboardFilter = {}): TaskDashboardFilter {
  return {
    scope: filters.scope,
    workspace: normalizeOptionalText(filters.workspace),
    owner_kind: filters.owner_kind,
    owner_ref: normalizeOptionalText(filters.owner_ref),
    participation_channel: normalizeOptionalText(filters.participation_channel),
    origin_kind: filters.origin_kind,
  };
}

export async function getTaskDashboard(
  filters: TaskDashboardFilter = {},
  signal?: AbortSignal
): Promise<TaskDashboardView> {
  const { data, error, response } = await apiClient.GET("/api/observe/tasks/dashboard", {
    params: { query: normalizeDashboardFilter(filters) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new TasksApiError(
      defaultApiErrorMessage("Failed to fetch task dashboard", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch task dashboard").dashboard;
}

export async function getTaskInbox(
  filters: TaskInboxFilter = {},
  signal?: AbortSignal
): Promise<TaskInboxView> {
  const { data, error, response } = await apiClient.GET("/api/observe/tasks/inbox", {
    params: { query: normalizeTaskInboxFilter(filters) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new TasksApiError(
      defaultApiErrorMessage("Failed to fetch task inbox", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch task inbox").inbox;
}
