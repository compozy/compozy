import { toast } from "@compozy/ui";
import { useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";

import { NetworkApiError, sendNetworkMessage } from "../adapters/network-api";
import { networkKeys } from "../lib/query-keys";
import type {
  CreateNetworkThreadInput,
  CreateNetworkThreadResult,
  ScopedSendNetworkMessageInput,
  UseCreateNetworkThreadOptions,
  UseCreateNetworkThreadResult,
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
  reconcileOptimisticMessage,
} from "./network-message-cache";
import { useProfileReadScope } from "@/systems/profiles";
import { useActiveWorkspace } from "@/systems/workspace";

export const THREAD_COLLISION_TOAST = "Couldn't open this thread. Try again.";

interface CreateThreadAttemptArgs extends CreateNetworkThreadInput {
  threadId: string;
  workspaceId: string;
  /** The profile the new thread's first message is filed under (ADR-005). */
  owner: { id: string; name: string };
}

async function attemptCreateThread(
  queryClient: QueryClient,
  args: CreateThreadAttemptArgs
): Promise<CreateNetworkThreadResult> {
  const clientMessageId = generateClientMessageId();
  const input: ScopedSendNetworkMessageInput = {
    ...args,
    surface: "thread",
  };
  const optimistic = buildOptimisticMessage(
    input,
    clientMessageId,
    new Date().toISOString(),
    args.owner
  );
  await queryClient.cancelQueries({ queryKey: canonicalNetworkMessageKey(input), exact: true });
  applyOptimisticMessage(queryClient, input, optimistic);
  try {
    const response = await sendNetworkMessage(
      args.workspaceId,
      buildSendRequest(input, clientMessageId, args.workspaceId)
    );
    reconcileOptimisticMessage(queryClient, input, clientMessageId, response.message);
    return {
      threadId: response.message.thread_id ?? args.threadId,
      rootMessageId: response.message.id,
    };
  } catch (error) {
    discardOptimisticMessage(queryClient, input, clientMessageId);
    throw error;
  }
}

export function useCreateNetworkThread(
  options: UseCreateNetworkThreadOptions = {}
): UseCreateNetworkThreadResult {
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const scope = useProfileReadScope();
  const owner = { id: scope.destinationOwner?.id ?? "", name: scope.destination };
  const workspaceId = options.workspaceId ?? runtimeWorkspaceId;
  const mutation = useMutation<
    CreateNetworkThreadResult,
    Error,
    CreateNetworkThreadInput & { workspaceId: string }
  >({
    mutationFn: async input => {
      const firstId = `thread_${generateClientMessageId().replace(/-/g, "")}`;
      try {
        return await attemptCreateThread(queryClient, { ...input, threadId: firstId, owner });
      } catch (firstError) {
        const secondId = `thread_${generateClientMessageId().replace(/-/g, "")}`;
        try {
          return await attemptCreateThread(queryClient, { ...input, threadId: secondId, owner });
        } catch (secondError) {
          toast.error(THREAD_COLLISION_TOAST);
          throw secondError instanceof Error ? secondError : firstError;
        }
      }
    },
    onSettled: (_data, _error, variables) => {
      void queryClient.invalidateQueries({
        queryKey: networkKeys.threadsRoot(variables.workspaceId, variables.channel),
      });
      void queryClient.invalidateQueries({
        queryKey: networkKeys.channelDetail(variables.workspaceId, variables.channel),
      });
      void queryClient.invalidateQueries({
        queryKey: networkKeys.channelsRoot(variables.workspaceId),
      });
    },
  });
  return {
    createThread: (input: CreateNetworkThreadInput) =>
      workspaceId
        ? mutation.mutateAsync({ ...input, workspaceId })
        : Promise.reject(new NetworkApiError("No active workspace selected", 400)),
    isCreating: mutation.isPending,
  };
}
