import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  TaskBridgeNotificationSubscription,
  TaskBridgeNotificationSubscriptionCreateRequest,
  TaskBridgeNotificationSubscriptionsFilter,
} from "../types";
import { normalizeOptionalText, TasksApiError } from "./tasks-api-errors";

function normalizeBridgeNotificationFilter(
  filters: TaskBridgeNotificationSubscriptionsFilter = {}
): TaskBridgeNotificationSubscriptionsFilter {
  return {
    bridge_instance_id: normalizeOptionalText(filters.bridge_instance_id),
    scope: filters.scope,
    workspace_id: normalizeOptionalText(filters.workspace_id),
    limit: filters.limit,
  };
}

export async function listTaskBridgeNotificationSubscriptions(
  taskId: string,
  filters: TaskBridgeNotificationSubscriptionsFilter = {},
  signal?: AbortSignal
): Promise<TaskBridgeNotificationSubscription[]> {
  const { data, error, response } = await apiClient.GET("/api/tasks/{id}/notifications/bridges", {
    params: { path: { id: taskId }, query: normalizeBridgeNotificationFilter(filters) },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) throw new TasksApiError(`Task not found: ${taskId}`, 404);
    throw new TasksApiError(
      defaultApiErrorMessage(
        `Failed to fetch bridge notification subscriptions for task "${taskId}"`,
        response,
        error
      ),
      response.status
    );
  }
  return requireResponseData(
    data,
    response,
    `Failed to fetch bridge notification subscriptions for task "${taskId}"`
  ).subscriptions;
}

export async function createTaskBridgeNotificationSubscription(
  taskId: string,
  body: TaskBridgeNotificationSubscriptionCreateRequest,
  signal?: AbortSignal
): Promise<TaskBridgeNotificationSubscription> {
  const { data, error, response } = await apiClient.POST("/api/tasks/{id}/notifications/bridges", {
    params: { path: { id: taskId } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new TasksApiError(`Task or bridge not found for task "${taskId}"`, 404);
    }
    throw new TasksApiError(
      defaultApiErrorMessage(
        `Failed to create bridge notification subscription for task "${taskId}"`,
        response,
        error
      ),
      response.status
    );
  }
  return requireResponseData(
    data,
    response,
    `Failed to create bridge notification subscription for task "${taskId}"`
  ).subscription;
}

export async function getTaskBridgeNotificationSubscription(
  taskId: string,
  subscriptionId: string,
  signal?: AbortSignal
): Promise<TaskBridgeNotificationSubscription> {
  const { data, error, response } = await apiClient.GET(
    "/api/tasks/{id}/notifications/bridges/{subscription_id}",
    {
      params: { path: { id: taskId, subscription_id: subscriptionId } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new TasksApiError(`Bridge notification subscription not found: ${subscriptionId}`, 404);
    }
    throw new TasksApiError(
      defaultApiErrorMessage(
        `Failed to fetch bridge notification subscription "${subscriptionId}"`,
        response,
        error
      ),
      response.status
    );
  }
  return requireResponseData(
    data,
    response,
    `Failed to fetch bridge notification subscription "${subscriptionId}"`
  ).subscription;
}

export async function deleteTaskBridgeNotificationSubscription(
  taskId: string,
  subscriptionId: string,
  signal?: AbortSignal
): Promise<void> {
  const { error, response } = await apiClient.DELETE(
    "/api/tasks/{id}/notifications/bridges/{subscription_id}",
    {
      params: { path: { id: taskId, subscription_id: subscriptionId } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new TasksApiError(`Bridge notification subscription not found: ${subscriptionId}`, 404);
    }
    throw new TasksApiError(
      defaultApiErrorMessage(
        `Failed to delete bridge notification subscription "${subscriptionId}"`,
        response,
        error
      ),
      response.status
    );
  }
}
