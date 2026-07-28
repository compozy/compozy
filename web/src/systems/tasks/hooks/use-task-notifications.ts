import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createTaskBridgeNotificationSubscription,
  deleteTaskBridgeNotificationSubscription,
} from "../adapters/tasks-api";
import {
  taskBridgeNotificationSubscriptionOptions,
  taskBridgeNotificationSubscriptionsOptions,
} from "../lib/query-options";
import { tasksKeys } from "../lib/query-keys";
import { acknowledgeTaskMutationSettlement } from "../lib/task-mutation";
import type {
  TaskBridgeNotificationSubscriptionCreateRequest,
  TaskBridgeNotificationSubscriptionsFilter,
} from "../types";

interface QueryHookOptions {
  enabled?: boolean;
}

interface CreateSubscriptionParams {
  taskId: string;
  data: TaskBridgeNotificationSubscriptionCreateRequest;
}

interface DeleteSubscriptionParams {
  taskId: string;
  subscriptionId: string;
}

type QueryClient = ReturnType<typeof useQueryClient>;

function invalidateBridgeNotificationQueries(
  queryClient: QueryClient,
  taskId: string,
  subscriptionId?: string
) {
  const pending: Promise<void>[] = [
    queryClient.invalidateQueries({ queryKey: tasksKeys.bridgeNotificationsRoot() }),
    queryClient.invalidateQueries({ queryKey: tasksKeys.detail(taskId) }),
    queryClient.invalidateQueries({ queryKey: tasksKeys.timelineRoot() }),
  ];

  if (subscriptionId) {
    pending.push(
      queryClient.invalidateQueries({
        queryKey: tasksKeys.bridgeNotification(taskId, subscriptionId),
      })
    );
  }

  return Promise.all(pending);
}

export function useTaskBridgeNotificationSubscriptions(
  taskId: string,
  filters: TaskBridgeNotificationSubscriptionsFilter = {},
  options: QueryHookOptions = {}
) {
  return useQuery(
    taskBridgeNotificationSubscriptionsOptions(taskId, filters, options.enabled ?? true)
  );
}

export function useTaskBridgeNotificationSubscription(
  taskId: string,
  subscriptionId: string,
  options: QueryHookOptions = {}
) {
  return useQuery(
    taskBridgeNotificationSubscriptionOptions(taskId, subscriptionId, options.enabled ?? true)
  );
}

export function useCreateTaskBridgeNotificationSubscription() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ taskId, data }: CreateSubscriptionParams) =>
      createTaskBridgeNotificationSubscription(taskId, data),
    onSettled: (result, error, { taskId }) => {
      acknowledgeTaskMutationSettlement(error);
      return invalidateBridgeNotificationQueries(queryClient, taskId, result?.subscription_id);
    },
  });
}

export function useDeleteTaskBridgeNotificationSubscription() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ taskId, subscriptionId }: DeleteSubscriptionParams) =>
      deleteTaskBridgeNotificationSubscription(taskId, subscriptionId),
    onSettled: (_result, error, { taskId, subscriptionId }) => {
      acknowledgeTaskMutationSettlement(error);
      queryClient.removeQueries({ queryKey: tasksKeys.bridgeNotification(taskId, subscriptionId) });
      return invalidateBridgeNotificationQueries(queryClient, taskId, subscriptionId);
    },
  });
}
