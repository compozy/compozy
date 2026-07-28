import type { InfiniteData } from "@tanstack/react-query";

import type {
  NetworkConversationMessage,
  NetworkConversationMessagesResponse,
  NetworkMessagePageParam,
} from "../types";
import { NETWORK_DEFAULT_TIMELINE_LIMIT } from "./query-constants";

export type NetworkMessageData = InfiniteData<
  NetworkConversationMessagesResponse,
  NetworkMessagePageParam
>;

function mergeMessages(
  current: ReadonlyArray<NetworkConversationMessage>,
  incoming: ReadonlyArray<NetworkConversationMessage>
): NetworkConversationMessage[] {
  const byId = new Map(current.map(message => [message.message_id, message]));
  for (const message of incoming) byId.set(message.message_id, message);
  return [...byId.values()];
}

export function rollNetworkMessageTail(
  previous: NetworkMessageData,
  incoming: NetworkConversationMessage[]
): NetworkMessageData {
  if (incoming.length === 0 || !previous.pages[0]) return previous;
  const pages = previous.pages.map(page => ({ ...page, messages: [...page.messages] }));
  pages[0]!.messages = mergeMessages(pages[0]!.messages, incoming);
  for (let index = 0; index < pages.length; index += 1) {
    const page = pages[index]!;
    const limit = Math.max(1, page.page.limit || NETWORK_DEFAULT_TIMELINE_LIMIT);
    if (page.messages.length <= limit) continue;
    const overflow = page.messages.slice(0, page.messages.length - limit);
    page.messages = page.messages.slice(-limit);
    const next = pages[index + 1];
    if (next) {
      next.messages = mergeMessages(next.messages, overflow);
    } else {
      pages.push({ messages: overflow, page: { ...page.page } });
    }
  }
  const pageParams = [...previous.pageParams];
  while (pageParams.length < pages.length) {
    const newerPage = pages[pageParams.length - 1];
    pageParams.push(newerPage?.messages[0] ? { before: newerPage.messages[0].message_id } : null);
  }
  return { pages, pageParams };
}

export function mergeNetworkTailResponse(
  previous: NetworkMessageData,
  response: NetworkConversationMessagesResponse,
  initialProbe: boolean
): NetworkMessageData {
  const next = rollNetworkMessageTail(previous, response.messages);
  if (!initialProbe || previous.pages.length !== 1 || next.pages.length === 0) return next;
  const lastIndex = next.pages.length - 1;
  const lastPage = next.pages[lastIndex];
  if (lastPage) next.pages[lastIndex] = { ...lastPage, page: response.page };
  return next;
}

export function latestNetworkTailId(data: NetworkMessageData | undefined): string | undefined {
  const messages = data?.pages[0]?.messages ?? [];
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (message && !("optimistic" in message)) return message.message_id;
  }
  return undefined;
}
