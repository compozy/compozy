import { useMutation, useQueryClient } from "@tanstack/react-query";

import { NetworkApiError, sendNetworkMessage } from "../adapters/network-api";
import { networkKeys } from "../lib/query-keys";
import type {
  ScopedSendNetworkMessageInput,
  SendNetworkMessageInput,
  SendNetworkMessageResult,
  UseSendNetworkMessageResult,
} from "./network-action-types";
import {
  buildOptimisticMessage,
  buildSendRequest,
  generateClientMessageId,
} from "./network-message-builders";
import {
  applyOptimisticMessage,
  canonicalNetworkMessageKey,
  discardOptimisticMessage,
  markOptimisticMessageFailed,
  reconcileOptimisticMessage,
  resetOptimisticMessage,
} from "./network-message-cache";
import { resolveNetworkMessageOwner, type NetworkMessageOwner } from "./network-message-owner";
import { sessionKeys } from "@/systems/session";
import { useProfileReadScope } from "@/systems/profiles";
import { useActiveWorkspace } from "@/systems/workspace";

function invalidateConversation(
  queryClient: ReturnType<typeof useQueryClient>,
  input: ScopedSendNetworkMessageInput
): void {
  const roots =
    input.surface === "thread"
      ? [
          networkKeys.threadDetail(input.workspaceId, input.channel, input.threadId),
          networkKeys.threadsRoot(input.workspaceId, input.channel),
        ]
      : [
          networkKeys.directDetail(input.workspaceId, input.channel, input.directId),
          networkKeys.directsRoot(input.workspaceId, input.channel),
        ];
  for (const queryKey of roots) void queryClient.invalidateQueries({ queryKey });
  void queryClient.invalidateQueries({ queryKey: networkKeys.channelsRoot(input.workspaceId) });
  void queryClient.invalidateQueries({
    queryKey: sessionKeys.workspaceLists(input.workspaceId),
  });
}

export function useSendNetworkMessage(
  options: { workspaceId?: string | null } = {}
): UseSendNetworkMessageResult {
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const scope = useProfileReadScope();
  const fallbackOwner: NetworkMessageOwner = {
    id: scope.destinationOwner?.id ?? "",
    name: scope.destination,
  };
  const workspaceId = options.workspaceId ?? runtimeWorkspaceId;
  const mutation = useMutation<SendNetworkMessageResult, Error, ScopedSendNetworkMessageInput>({
    mutationFn: async input => {
      const clientMessageId = input.clientMessageId ?? generateClientMessageId();
      const owner = resolveNetworkMessageOwner(
        queryClient,
        input.workspaceId,
        input.channel,
        fallbackOwner
      );
      if (owner) {
        const optimistic = buildOptimisticMessage(
          input,
          clientMessageId,
          new Date().toISOString(),
          owner
        );
        await queryClient.cancelQueries({
          queryKey: canonicalNetworkMessageKey(input),
          exact: true,
        });
        if (input.clientMessageId) resetOptimisticMessage(queryClient, input, optimistic);
        else applyOptimisticMessage(queryClient, input, optimistic);
      }
      try {
        const response = await sendNetworkMessage(
          input.workspaceId,
          buildSendRequest(input, clientMessageId, input.workspaceId)
        );
        if (owner)
          reconcileOptimisticMessage(queryClient, input, clientMessageId, response.message);
        return { clientMessageId, response };
      } catch (error) {
        if (owner) markOptimisticMessageFailed(queryClient, input, clientMessageId);
        throw error;
      }
    },
    onSettled: (_data, _error, variables) => invalidateConversation(queryClient, variables),
  });
  const scoped = (input: SendNetworkMessageInput, clientMessageId?: string) => {
    if (!workspaceId) {
      return Promise.reject(new NetworkApiError("No active workspace selected", 400));
    }
    return mutation.mutateAsync({ ...input, clientMessageId, workspaceId });
  };
  const discard = (input: SendNetworkMessageInput, clientMessageId: string) => {
    if (workspaceId)
      discardOptimisticMessage(queryClient, { ...input, workspaceId }, clientMessageId);
  };
  return {
    send: (input: SendNetworkMessageInput) => scoped(input),
    retry: (input: SendNetworkMessageInput, clientMessageId: string) =>
      scoped(input, clientMessageId),
    discard,
    isSending: mutation.isPending,
  };
}
