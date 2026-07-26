import type { InfiniteData, QueryClient, QueryKey } from "@tanstack/react-query";

import {
  NETWORK_DEFAULT_TIMELINE_LIMIT,
  networkDirectMessagesOptions,
  networkThreadMessagesOptions,
} from "../lib/query-options";
import type {
  NetworkConversationMessage,
  NetworkConversationMessagesResponse,
  NetworkMessagePageParam,
  NetworkSendResponse,
} from "../types";
import type {
  OptimisticConversationMessage,
  ScopedSendNetworkMessageInput,
} from "./network-action-types";

type MessageData = InfiniteData<NetworkConversationMessagesResponse, NetworkMessagePageParam>;

export function isOptimisticMessage(
  message: NetworkConversationMessage
): message is OptimisticConversationMessage {
  return (message as Partial<OptimisticConversationMessage>).optimistic != null;
}

export function canonicalNetworkMessageKey(input: ScopedSendNetworkMessageInput): QueryKey {
  if (input.surface === "thread") {
    return networkThreadMessagesOptions(input.workspaceId, input.channel, input.threadId).queryKey;
  }
  return networkDirectMessagesOptions(input.workspaceId, input.channel, input.directId).queryKey;
}

function updateCanonicalTail(
  queryClient: QueryClient,
  input: ScopedSendNetworkMessageInput,
  update: (messages: NetworkConversationMessage[]) => NetworkConversationMessage[],
  seedWhenMissing = false
): void {
  queryClient.setQueryData<MessageData>(canonicalNetworkMessageKey(input), previous => {
    if (!previous) {
      if (!seedWhenMissing) return previous;
      return {
        pages: [
          {
            messages: update([]),
            page: { has_more: false, limit: NETWORK_DEFAULT_TIMELINE_LIMIT },
          },
        ],
        pageParams: [null],
      };
    }
    const first = previous.pages[0];
    if (!first) return previous;
    return {
      ...previous,
      pages: [{ ...first, messages: update(first.messages) }, ...previous.pages.slice(1)],
    };
  });
}

export function applyOptimisticMessage(
  queryClient: QueryClient,
  input: ScopedSendNetworkMessageInput,
  optimistic: OptimisticConversationMessage
): void {
  updateCanonicalTail(queryClient, input, messages => [...messages, optimistic], true);
}

export function resetOptimisticMessage(
  queryClient: QueryClient,
  input: ScopedSendNetworkMessageInput,
  optimistic: OptimisticConversationMessage
): void {
  updateCanonicalTail(queryClient, input, messages =>
    messages.map(message => (message.message_id === optimistic.message_id ? optimistic : message))
  );
}

function canonicalMessage(
  optimistic: NetworkConversationMessage,
  response: NetworkSendResponse["message"]
): NetworkConversationMessage {
  const canonical: NetworkConversationMessage = {
    ...optimistic,
    causation_id: response.causation_id,
    channel: response.channel,
    direct_id: response.direct_id,
    kind: response.kind,
    mentions: response.mentions,
    message_id: response.id,
    peer_to: response.to,
    reply_to: response.reply_to,
    session_id: response.session_id,
    surface: response.surface,
    thread_id: response.thread_id,
    trace_id: response.trace_id,
    work_id: response.work_id,
    workspace_id: response.workspace_id,
  };
  if (isOptimisticMessage(canonical)) {
    delete (canonical as Partial<OptimisticConversationMessage>).optimistic;
  }
  return canonical;
}

export function reconcileOptimisticMessage(
  queryClient: QueryClient,
  input: ScopedSendNetworkMessageInput,
  clientMessageId: string,
  response: NetworkSendResponse["message"]
): void {
  updateCanonicalTail(queryClient, input, messages =>
    messages.map(message =>
      message.message_id === clientMessageId ? canonicalMessage(message, response) : message
    )
  );
}

export function markOptimisticMessageFailed(
  queryClient: QueryClient,
  input: ScopedSendNetworkMessageInput,
  clientMessageId: string
): void {
  updateCanonicalTail(queryClient, input, messages =>
    messages.map(message =>
      message.message_id === clientMessageId
        ? ({ ...message, optimistic: "failed" } as OptimisticConversationMessage)
        : message
    )
  );
}

export function discardOptimisticMessage(
  queryClient: QueryClient,
  input: ScopedSendNetworkMessageInput,
  clientMessageId: string
): void {
  updateCanonicalTail(queryClient, input, messages =>
    messages.filter(message => message.message_id !== clientMessageId)
  );
}
