import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useSelector, useStore } from "@xstate/store-react";

import { createStreamEventSource } from "@/lib/ticketed-event-source";

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
  const store = useStore(taskRunConversationLogic, { scope: usageScope });
  const usageState = useSelector(store, snapshot => snapshot.context);
  const messages = useNetworkMessages({
    workspaceId: conversation?.workspace_id,
    channel: conversation?.channel,
    surface: conversation?.surface === "thread" ? "thread" : null,
    containerId: conversation?.thread_id,
    enabled: enabled && Boolean(network),
  });
  const usage =
    usageState.scope === usageScope && usageState.usageOverlay
      ? usageState.usageOverlay
      : (network?.usage ?? null);

  useEffect(() => {
    store.trigger.networkObserved({ scope: usageScope });
  }, [store, usageScope]);

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
        if (payload.usage) {
          store.trigger.usageReceived({ scope: usageScope, usage: payload.usage });
        }
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
    store,
    usageScope,
  ]);

  if (!network || !usage) return null;
  return { ...messages, usage, streamError: usageState.streamError };
}
