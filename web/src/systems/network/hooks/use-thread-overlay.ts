import { useEffect } from "react";

import type { NetworkConversationMessage, NetworkThreadDetail } from "../types";
import { useLastRead } from "./use-last-read";
import { useNetworkMessages } from "./use-messages";
import { useNetworkThreadDetail } from "./use-threads";

export interface UseThreadOverlayArgs {
  workspaceId: string;
  channel: string;
  threadId: string;
  fullPage: boolean;
  onClose: () => void;
}

export interface UseThreadOverlayResult {
  detail: NetworkThreadDetail | null;
  isDetailLoading: boolean;
  detailError: Error | null;
  rootMessage: NetworkConversationMessage | null;
  replies: NetworkConversationMessage[];
  replyCount: number;
  isMessagesLoading: boolean;
  hasOlder: boolean;
  isLoadingOlder: boolean;
  loadOlder: () => Promise<void>;
  messagesError: Error | null;
  lastReadIso: string | null;
}

function pickRoot(
  detail: NetworkThreadDetail | null,
  messages: ReadonlyArray<NetworkConversationMessage>
): NetworkConversationMessage | null {
  if (!detail) {
    return null;
  }
  const rootId = detail.root_message_id;
  if (rootId == null) {
    return messages[0] ?? null;
  }
  return messages.find(message => message.message_id === rootId) ?? null;
}

export function useThreadOverlay({
  workspaceId,
  channel,
  threadId,
  fullPage,
  onClose,
}: UseThreadOverlayArgs): UseThreadOverlayResult {
  const detail = useNetworkThreadDetail(channel, threadId, { workspaceId });
  const messagesQuery = useNetworkMessages({
    workspaceId,
    channel,
    containerId: threadId,
    enabled: Boolean(detail.thread),
    surface: "thread",
  });
  const { lastReadAt, markRead } = useLastRead({ workspaceId });
  const lastReadIso = lastReadAt({ channel, containerId: threadId, surface: "thread" });

  const rootMessage = pickRoot(detail.thread, messagesQuery.messages);
  const replies = rootMessage
    ? messagesQuery.messages.filter(message => message.message_id !== rootMessage.message_id)
    : [...messagesQuery.messages];
  const replyCount = Math.max(
    0,
    (detail.thread?.message_count ?? messagesQuery.messages.length) - 1
  );

  useEffect(() => {
    if (fullPage) {
      return undefined;
    }
    function handleKey(event: KeyboardEvent) {
      if (event.key !== "Escape") {
        return;
      }
      onClose();
    }
    window.addEventListener("keydown", handleKey);
    return () => window.removeEventListener("keydown", handleKey);
  }, [fullPage, onClose]);

  useEffect(() => {
    const lastTimestamp = messagesQuery.messages.at(-1)?.timestamp;
    if (!lastTimestamp) {
      return;
    }
    markRead({ channel, containerId: threadId, surface: "thread" }, lastTimestamp);
  }, [channel, markRead, messagesQuery.messages, threadId]);

  return {
    detail: detail.thread,
    isDetailLoading: detail.isLoading,
    detailError: detail.error,
    rootMessage,
    replies,
    replyCount,
    isMessagesLoading: messagesQuery.isLoading,
    hasOlder: messagesQuery.hasOlder,
    isLoadingOlder: messagesQuery.isLoadingOlder,
    loadOlder: messagesQuery.loadOlder,
    messagesError: messagesQuery.error,
    lastReadIso,
  };
}
