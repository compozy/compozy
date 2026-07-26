import { toast } from "@agh/ui";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { sessionKeys } from "@/systems/session";
import { tasksKeys } from "@/systems/tasks";
import { useActiveWorkspace } from "@/systems/workspace";

import {
  createNetworkChannel,
  deleteNetworkSubscription,
  NetworkApiError,
  promoteNetworkThreadTask,
  updateNetworkChannel,
  upsertNetworkSubscription,
} from "../adapters/network-api";
import { networkKeys } from "../lib/query-keys";
import type {
  CreateNetworkChannelRequest,
  NetworkChannelUpdateResponse,
  NetworkSubscriptionMode,
  NetworkSubscriptionResponse,
  PromoteNetworkThreadTaskResponse,
} from "../types";
import type {
  DeleteNetworkSubscriptionInput,
  PromoteNetworkThreadTaskInput,
  UpdateNetworkChannelInput,
  UpsertNetworkSubscriptionInput,
} from "./network-action-types";

export function useCreateNetworkChannel(options: { workspaceId?: string | null } = {}) {
  const queryClient = useQueryClient();
  const { activeWorkspaceId } = useActiveWorkspace();
  const requestedWorkspaceId = options.workspaceId ?? activeWorkspaceId;
  return useMutation({
    mutationFn: (data: CreateNetworkChannelRequest) => {
      const workspaceId = requestedWorkspaceId ?? data.workspace_id;
      return workspaceId
        ? createNetworkChannel(workspaceId, data)
        : Promise.reject(new NetworkApiError("No active workspace selected", 400));
    },
    onSettled: (_data, _error, variables) => {
      const workspaceId = requestedWorkspaceId ?? variables.workspace_id;
      return Promise.all([
        queryClient.invalidateQueries({
          queryKey: networkKeys.channelsRoot(workspaceId),
        }),
        queryClient.invalidateQueries({
          queryKey: sessionKeys.workspaceLists(workspaceId),
        }),
      ]);
    },
  });
}

export function useUpdateNetworkChannel() {
  const queryClient = useQueryClient();
  return useMutation<NetworkChannelUpdateResponse["channel"], Error, UpdateNetworkChannelInput>({
    mutationFn: ({ workspaceId, channel, data }) =>
      updateNetworkChannel(workspaceId, channel, data),
    onSuccess: (_channel, { workspaceId, channel }) => {
      toast.success("Channel policy updated.");
      return Promise.all([
        queryClient.invalidateQueries({ queryKey: networkKeys.channelsRoot(workspaceId) }),
        queryClient.invalidateQueries({
          queryKey: networkKeys.channelDetail(workspaceId, channel),
        }),
        queryClient.invalidateQueries({ queryKey: sessionKeys.workspaceLists(workspaceId) }),
      ]);
    },
    onError: error => toast.error(error.message || "Failed to update channel policy"),
  });
}

export function useUpsertNetworkSubscription() {
  const queryClient = useQueryClient();
  return useMutation<
    NetworkSubscriptionResponse["subscription"],
    Error,
    UpsertNetworkSubscriptionInput
  >({
    mutationFn: ({ workspaceId, channel, data }) =>
      upsertNetworkSubscription(workspaceId, channel, data),
    onSuccess: (_subscription, { workspaceId, channel, data }) => {
      toast.success(`Thread delivery set to ${data.mode as NetworkSubscriptionMode}.`);
      return queryClient.invalidateQueries({
        queryKey: networkKeys.subscriptionsRoot(workspaceId, channel),
      });
    },
    onError: error => toast.error(error.message || "Failed to update subscription"),
  });
}

export function useDeleteNetworkSubscription() {
  const queryClient = useQueryClient();
  return useMutation<void, Error, DeleteNetworkSubscriptionInput>({
    mutationFn: ({ workspaceId, channel, sessionId, threadId }) =>
      deleteNetworkSubscription(workspaceId, channel, sessionId, { thread_id: threadId }),
    onSuccess: (_result, { workspaceId, channel }) => {
      toast.success("Thread delivery reset.");
      return queryClient.invalidateQueries({
        queryKey: networkKeys.subscriptionsRoot(workspaceId, channel),
      });
    },
    onError: error => toast.error(error.message || "Failed to reset subscription"),
  });
}

export function usePromoteNetworkThreadTask() {
  const queryClient = useQueryClient();
  return useMutation<PromoteNetworkThreadTaskResponse, Error, PromoteNetworkThreadTaskInput>({
    mutationFn: ({ workspaceId, channel, threadId, data }) =>
      promoteNetworkThreadTask(workspaceId, channel, threadId, data),
    onSuccess: (result, { workspaceId, channel, threadId }) => {
      toast.success("Thread promoted to task.");
      return Promise.all([
        queryClient.invalidateQueries({
          queryKey: networkKeys.threadDetail(workspaceId, channel, threadId),
        }),
        queryClient.invalidateQueries({ queryKey: networkKeys.threadsRoot(workspaceId, channel) }),
        queryClient.invalidateQueries({ queryKey: tasksKeys.lists() }),
        queryClient.invalidateQueries({ queryKey: tasksKeys.detail(result.task.id) }),
      ]);
    },
    onError: error => toast.error(error.message || "Failed to promote thread"),
  });
}
