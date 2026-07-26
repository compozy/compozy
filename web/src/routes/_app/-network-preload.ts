import type { QueryClient } from "@tanstack/react-query";

import {
  networkChannelDetailOptions,
  networkChannelsOptions,
  networkDirectDetailOptions,
  networkDirectMessagesOptions,
  networkDirectsOptions,
  networkPeersOptions,
  networkStatusOptions,
  networkThreadDetailOptions,
  networkThreadMessagesOptions,
  networkThreadsOptions,
} from "@/systems/network";

import { settleRouteQueries } from "./-route-preload";

export async function preloadNetworkRootRoute(queryClient: QueryClient): Promise<void> {
  await settleRouteQueries([queryClient.ensureQueryData(networkStatusOptions())]);
}

/** Warms the Network index projection for restored and unfocused windows. */
export async function preloadNetworkWindowRoute(
  queryClient: QueryClient,
  workspaceId: string
): Promise<void> {
  await settleRouteQueries([
    queryClient.ensureQueryData(networkStatusOptions()),
    queryClient.ensureQueryData(networkChannelsOptions(workspaceId)),
  ]);
}

function channelQueries(queryClient: QueryClient, workspaceId: string, channel: string) {
  return [
    queryClient.ensureQueryData(networkStatusOptions()),
    queryClient.ensureQueryData(networkChannelsOptions(workspaceId)),
    queryClient.ensureQueryData(networkChannelDetailOptions(workspaceId, channel)),
  ];
}

export async function preloadNetworkThreadsRoute(
  queryClient: QueryClient,
  workspaceId: string,
  channel: string
): Promise<void> {
  await settleRouteQueries([
    ...channelQueries(queryClient, workspaceId, channel),
    queryClient.ensureInfiniteQueryData(networkThreadsOptions(workspaceId, channel)),
  ]);
}

export async function preloadNetworkThreadDetailRoute(
  queryClient: QueryClient,
  workspaceId: string,
  channel: string,
  threadId: string
): Promise<void> {
  await settleRouteQueries([
    ...channelQueries(queryClient, workspaceId, channel),
    queryClient.ensureQueryData(networkThreadDetailOptions(workspaceId, channel, threadId)),
    queryClient.ensureInfiniteQueryData(
      networkThreadMessagesOptions(workspaceId, channel, threadId)
    ),
  ]);
}

export async function preloadNetworkDirectsRoute(
  queryClient: QueryClient,
  workspaceId: string,
  channel: string
): Promise<void> {
  await settleRouteQueries([
    ...channelQueries(queryClient, workspaceId, channel),
    queryClient.ensureInfiniteQueryData(networkDirectsOptions(workspaceId, channel)),
    queryClient.ensureQueryData(networkPeersOptions(workspaceId, channel)),
  ]);
}

export async function preloadNetworkDirectDetailRoute(
  queryClient: QueryClient,
  workspaceId: string,
  channel: string,
  directId: string
): Promise<void> {
  await settleRouteQueries([
    ...channelQueries(queryClient, workspaceId, channel),
    queryClient.ensureQueryData(networkDirectDetailOptions(workspaceId, channel, directId)),
    queryClient.ensureInfiniteQueryData(
      networkDirectMessagesOptions(workspaceId, channel, directId)
    ),
    queryClient.ensureQueryData(networkPeersOptions(workspaceId, channel)),
  ]);
}

export async function preloadNetworkActivityRoute(
  queryClient: QueryClient,
  workspaceId: string,
  channel: string
): Promise<void> {
  await settleRouteQueries([
    ...channelQueries(queryClient, workspaceId, channel),
    queryClient.ensureInfiniteQueryData(networkThreadsOptions(workspaceId, channel)),
    queryClient.ensureInfiniteQueryData(networkDirectsOptions(workspaceId, channel)),
  ]);
}
