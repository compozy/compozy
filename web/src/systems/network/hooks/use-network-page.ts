import { useQuery } from "@tanstack/react-query";

import { networkStatusOptions } from "../lib/query-options";
import type { NetworkChannelSummary, NetworkRecentEntry, NetworkStatus } from "../types";
import { useNetworkChannels } from "./use-channels";
import { useNetworkRecents } from "./use-recents";

export interface UseNetworkPageResult {
  status: NetworkStatus | null;
  isStatusLoading: boolean;
  statusError: Error | null;
  isNetworkEnabled: boolean;
  isNetworkDisabled: boolean;
  channels: NetworkChannelSummary[];
  pinnedChannels: NetworkChannelSummary[];
  unpinnedChannels: NetworkChannelSummary[];
  pinnedChannelIds: ReadonlyArray<string>;
  isPinned: (channel: string) => boolean;
  togglePinned: (channel: string) => void;
  isChannelsLoading: boolean;
  channelsError: Error | null;
  recents: NetworkRecentEntry[];
  isRecentsLoading: boolean;
  firstVisibleChannel: NetworkChannelSummary | null;
}

export function useNetworkPage(
  workspaceId?: string | null,
  options: { enabled?: boolean } = {}
): UseNetworkPageResult {
  const statusQuery = useQuery(networkStatusOptions());
  const status = statusQuery.data ?? null;
  const isNetworkEnabled = status?.enabled === true;
  const isNetworkDisabled = status?.enabled === false;
  const enabled = isNetworkEnabled && (options.enabled ?? true);
  const channelsResult = useNetworkChannels({ workspaceId, enabled });
  const recentsResult = useNetworkRecents(channelsResult.recents, {
    workspaceId,
    enabled,
    isLoading: channelsResult.isLoading,
  });
  const firstVisibleChannel: NetworkChannelSummary | null =
    channelsResult.pinned[0] ?? channelsResult.unpinned[0] ?? null;
  return {
    status,
    isStatusLoading: statusQuery.isLoading && !status,
    statusError: statusQuery.error ?? null,
    isNetworkEnabled,
    isNetworkDisabled,
    channels: channelsResult.channels,
    pinnedChannels: channelsResult.pinned,
    unpinnedChannels: channelsResult.unpinned,
    pinnedChannelIds: channelsResult.pinnedIds,
    isPinned: channelsResult.isPinned,
    togglePinned: channelsResult.togglePinned,
    isChannelsLoading: channelsResult.isLoading,
    channelsError: channelsResult.error,
    recents: recentsResult.recents,
    isRecentsLoading: recentsResult.isLoading,
    firstVisibleChannel,
  };
}
