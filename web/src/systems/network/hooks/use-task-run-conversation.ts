import { useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useSelector } from "@xstate/store-react";

import { createStreamEventSource } from "@/lib/ticketed-event-source";
import { useStoreBinding } from "@/hooks/use-store-binding";
import { taskRunDetailOptions, type TaskRunDetailView } from "@/systems/tasks";

import { networkKeys } from "../lib/query-keys";
import type { TaskRunNetworkProjection, TaskRunNetworkUsage } from "../types";
import { taskRunConversationLogic } from "./task-run-conversation-store";
import { useNetworkMessages, type UseNetworkMessagesResult } from "./use-messages";

export interface UseTaskRunConversationResult extends UseNetworkMessagesResult {
  usage: TaskRunNetworkUsage;
  streamError: Error | null;
}

export function useTaskRunConversation(
  runId: string,
  network: TaskRunNetworkProjection | null | undefined,
  options: { enabled?: boolean } = {}
): UseTaskRunConversationResult | null {
  const queryClient = useQueryClient();
  const enabled = options.enabled ?? true;
  const conversation = network?.conversation;
  const conversationChannel = conversation?.channel ?? "";
  const conversationStreamUrl = conversation?.stream_url ?? "";
  const conversationThreadId = conversation?.thread_id ?? "";
  const conversationWorkspaceId = conversation?.workspace_id ?? "";
  const usageScope = network ? `${network.conversation.workspace_id}:${runId.trim()}` : "";
  const taskRun = useQuery(taskRunDetailOptions(runId, false)).data;
  const { store } = useStoreBinding(usageScope, () =>
    taskRunConversationLogic.createStore({ scope: usageScope })
  );
  const usageState = useSelector(store, snapshot => snapshot.context);
  const messages = useNetworkMessages({
    workspaceId: conversation?.workspace_id,
    channel: conversation?.channel,
    surface: conversation?.surface === "thread" ? "thread" : null,
    containerId: conversation?.thread_id,
    enabled: enabled && Boolean(network),
  });
  const usage = taskRun?.network?.usage ?? null;

  useEffect(() => {
    if (
      !enabled ||
      !conversationStreamUrl ||
      !conversationWorkspaceId ||
      typeof EventSource === "undefined"
    ) {
      store.trigger.streamSuspended({ scope: usageScope });
      return;
    }
    const source = createStreamEventSource(conversationStreamUrl);
    const handleMessage = () => {
      store.trigger.frameReceived({ scope: usageScope });
      void queryClient.invalidateQueries({
        queryKey: networkKeys.threadMessagesRoot(
          conversationWorkspaceId,
          conversationChannel,
          conversationThreadId
        ),
      });
    };
    const handleUsage = (event: MessageEvent<string>) => {
      try {
        const payload = JSON.parse(event.data) as { usage?: TaskRunNetworkUsage };
        const usage = payload.usage;
        if (!usage || usage.workspace_id !== conversationWorkspaceId) return;

        store.trigger.frameReceived({ scope: usageScope });
        queryClient.setQueryData<TaskRunDetailView>(
          taskRunDetailOptions(runId).queryKey,
          current => {
            if (
              !current?.network ||
              current.network.conversation.workspace_id !== conversationWorkspaceId ||
              current.network.usage.workspace_id !== usage.workspace_id
            ) {
              return current;
            }
            return {
              ...current,
              network: { ...current.network, usage },
            };
          }
        );
      } catch (error) {
        store.trigger.streamFailed({
          error:
            error instanceof Error ? error : new Error("Invalid task run conversation usage event"),
          scope: usageScope,
        });
      }
    };
    const handleError = () => {
      store.trigger.streamFailed({
        error: new Error("Live conversation updates are reconnecting"),
        scope: usageScope,
      });
    };
    source.addEventListener("network.message", handleMessage);
    source.addEventListener("network.usage", handleUsage as EventListener);
    source.addEventListener("error", handleError);
    return () => {
      source.removeEventListener("network.message", handleMessage);
      source.removeEventListener("network.usage", handleUsage as EventListener);
      source.removeEventListener("error", handleError);
      source.close();
    };
  }, [
    conversationChannel,
    conversationStreamUrl,
    conversationThreadId,
    conversationWorkspaceId,
    enabled,
    queryClient,
    runId,
    store,
    usageScope,
  ]);

  if (!network || !usage) return null;
  return { ...messages, usage, streamError: usageState.streamError };
}
