import type { InfiniteData } from "@tanstack/react-query";

import type {
  NetworkConversationMessage,
  NetworkConversationMessagesResponse,
  NetworkDirectRoomSummary,
  NetworkDirectRoomsResponse,
  NetworkThreadSummary,
  NetworkThreadsResponse,
} from "../types";

function uniqueById<T>(items: ReadonlyArray<T>, id: (item: T) => string): T[] {
  const unique = new Map<string, T>();
  for (const item of items) {
    unique.set(id(item), item);
  }
  return [...unique.values()];
}

export function flattenNetworkThreads(
  data: InfiniteData<NetworkThreadsResponse, unknown> | undefined
): NetworkThreadSummary[] {
  return uniqueById(data?.pages.flatMap(page => page.threads) ?? [], thread => thread.thread_id);
}

export function flattenNetworkDirects(
  data: InfiniteData<NetworkDirectRoomsResponse, unknown> | undefined
): NetworkDirectRoomSummary[] {
  return uniqueById(data?.pages.flatMap(page => page.directs) ?? [], direct => direct.direct_id);
}

export function flattenNetworkMessages<TPage extends NetworkConversationMessagesResponse>(
  data: InfiniteData<TPage, unknown> | undefined
): NetworkConversationMessage[] {
  const byId = new Map<string, NetworkConversationMessage>();
  const pages = data?.pages ?? [];
  for (let index = pages.length - 1; index >= 0; index -= 1) {
    for (const message of pages[index]?.messages ?? []) {
      byId.set(message.message_id, message);
    }
  }
  return [...byId.values()];
}
