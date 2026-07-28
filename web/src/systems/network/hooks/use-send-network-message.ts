import { useMutation, useQueryClient } from "@tanstack/react-query";

import { sessionKeys } from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";

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
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = options.workspaceId ?? activeWorkspaceId;
  const mutation = useMutation<SendNetworkMessageResult, Error, ScopedSendNetworkMessageInput>({
    mutationFn: async input => {
      const clientMessageId = input.clientMessageId ?? generateClientMessageId();
      const optimistic = buildOptimisticMessage(input, clientMessageId, new Date().toISOString());
      await queryClient.cancelQueries({
        queryKey: canonicalNetworkMessageKey(input),
        exact: true,
      });
      if (input.clientMessageId) resetOptimisticMessage(queryClient, input, optimistic);
      else applyOptimisticMessage(queryClient, input, optimistic);
      try {
        const response = await sendNetworkMessage(
          input.workspaceId,
          buildSendRequest(input, clientMessageId, input.workspaceId)
        );
        reconcileOptimisticMessage(queryClient, input, clientMessageId, response.message);
        return { clientMessageId, response };
      } catch (error) {
        markOptimisticMessageFailed(queryClient, input, clientMessageId);
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
