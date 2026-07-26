import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { networkKeys } from "../lib/query-keys";
import type { TaskRunNetworkProjection, TaskRunNetworkUsage } from "../types";
import { useNetworkMessages, type UseNetworkMessagesResult } from "./use-messages";

export interface UseTaskRunConversationResult extends UseNetworkMessagesResult {
  usage: TaskRunNetworkUsage;
  streamError: Error | null;
}

export function useTaskRunConversation(
  runId: string,
  network: TaskRunNetworkProjection | null | undefined
): UseTaskRunConversationResult | null {
  const queryClient = useQueryClient();
  const conversation = network?.conversation;
  const messages = useNetworkMessages({
    workspaceId: conversation?.workspace_id,
    channel: conversation?.channel,
    surface: conversation?.surface === "thread" ? "thread" : null,
    containerId: conversation?.thread_id,
    enabled: Boolean(network),
  });
  const usageScope = network ? `${network.conversation.workspace_id}:${runId.trim()}` : "";
  const [usageOverlay, setUsageOverlay] = useState<{
    scope: string;
    usage: TaskRunNetworkUsage;
  } | null>(null);
  const [streamError, setStreamError] = useState<Error | null>(null);
  const usage = usageOverlay?.scope === usageScope ? usageOverlay.usage : (network?.usage ?? null);

  useEffect(() => {
    if (!conversation || !network || typeof EventSource === "undefined") return;
    let active = true;
    const source = new EventSource(conversation.stream_url);
    const handleMessage = () => {
      setStreamError(null);
      void queryClient.invalidateQueries({
        queryKey: networkKeys.threadMessagesRoot(
          conversation.workspace_id,
          conversation.channel,
          conversation.thread_id
        ),
      });
    };
    const handleUsage = (event: MessageEvent<string>) => {
      try {
        const payload = JSON.parse(event.data) as { usage?: TaskRunNetworkUsage };
        if (payload.usage && active) {
          setUsageOverlay({ scope: usageScope, usage: payload.usage });
        }
        setStreamError(null);
      } catch (error) {
        setStreamError(
          error instanceof Error ? error : new Error("Invalid task run conversation usage event")
        );
      }
    };
    const handleError = () => {
      setStreamError(new Error("Live conversation updates are reconnecting"));
    };
    source.addEventListener("network.message", handleMessage);
    source.addEventListener("network.usage", handleUsage as EventListener);
    source.addEventListener("error", handleError);
    return () => {
      active = false;
      source.removeEventListener("network.message", handleMessage);
      source.removeEventListener("network.usage", handleUsage as EventListener);
      source.removeEventListener("error", handleError);
      source.close();
    };
  }, [conversation, network, queryClient, usageScope]);

  if (!network || !usage) return null;
  return { ...messages, usage, streamError };
}
