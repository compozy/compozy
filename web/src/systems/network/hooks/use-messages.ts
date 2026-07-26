import { useInfiniteQuery } from "@tanstack/react-query";

import { useActiveWorkspace } from "@/systems/workspace";

import { flattenNetworkMessages } from "../lib/infinite-data";
import { networkDirectMessagesOptions, networkThreadMessagesOptions } from "../lib/query-options";
import type {
  NetworkConversationMessage,
  NetworkConversationMessageFilters,
  NetworkSurface,
} from "../types";
import { useNetworkMessageTail } from "./use-network-message-tail";

export interface UseNetworkMessagesResult {
  messages: NetworkConversationMessage[];
  isLoading: boolean;
  isFetching: boolean;
  hasOlder: boolean;
  isLoadingOlder: boolean;
  loadOlder: () => Promise<void>;
  error: Error | null;
}

export interface UseNetworkMessagesArgs {
  workspaceId?: string | null;
  channel: string | null | undefined;
  surface: NetworkSurface | null | undefined;
  containerId: string | null | undefined;
  query?: NetworkConversationMessageFilters;
  enabled?: boolean;
}

export function useNetworkMessages({
  workspaceId: requestedWorkspaceId,
  channel,
  surface,
  containerId,
  query = {},
  enabled = true,
}: UseNetworkMessagesArgs): UseNetworkMessagesResult {
  const { activeWorkspaceId } = useActiveWorkspace();
  const selectedWorkspaceId = requestedWorkspaceId ?? activeWorkspaceId;
  const workspaceId = selectedWorkspaceId ?? "";
  const isReady =
    Boolean(channel) &&
    Boolean(surface) &&
    Boolean(containerId) &&
    enabled &&
    Boolean(selectedWorkspaceId);
  const threadQuery = useInfiniteQuery(
    networkThreadMessagesOptions(
      workspaceId,
      channel ?? "",
      containerId ?? "",
      query,
      isReady && surface === "thread"
    )
  );
  const directQuery = useInfiniteQuery(
    networkDirectMessagesOptions(
      workspaceId,
      channel ?? "",
      containerId ?? "",
      query,
      isReady && surface === "direct"
    )
  );
  const active = surface === "thread" ? threadQuery : directQuery;
  const tail = useNetworkMessageTail({
    workspaceId,
    channel: channel ?? "",
    surface,
    containerId: containerId ?? "",
    filters: query,
    enabled: isReady && Boolean(active.data) && !active.isFetchingNextPage,
  });
  const loadOlder = async () => {
    if (!active.hasNextPage || active.isFetchingNextPage) {
      return;
    }
    await active.fetchNextPage();
  };

  return {
    messages:
      surface === "thread"
        ? flattenNetworkMessages(threadQuery.data)
        : flattenNetworkMessages(directQuery.data),
    isLoading: isReady && active.isLoading,
    isFetching: active.isFetching || tail.isFetching,
    hasOlder: active.hasNextPage,
    isLoadingOlder: active.isFetchingNextPage,
    loadOlder,
    error: active.error ?? tail.error,
  };
}
