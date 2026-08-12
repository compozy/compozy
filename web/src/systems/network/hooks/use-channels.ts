import { useQuery } from "@tanstack/react-query";
import { useEffect } from "react";
import { useSelector } from "@xstate/store-react";

import { useStoreBinding } from "@/hooks/use-store-binding";
import { useNetworkLiveDataEnabled } from "./use-network-live-data-enabled";

import { NETWORK_DEFAULT_RECENTS_LIMIT, networkChannelsOptions } from "../lib/query-options";
import type { NetworkChannelSummary, NetworkChannelsResponse } from "../types";
import {
  createPinnedChannelsStore,
  PINNED_CHANNELS_STORAGE_KEY,
  rehydratePinnedChannelsStore,
} from "./pinned-channels-store";
import { useActiveWorkspace } from "@/systems/workspace";

export interface UseNetworkChannelsResult {
  channels: NetworkChannelSummary[];
  recents: NonNullable<NetworkChannelsResponse["recents"]>;
  pinned: NetworkChannelSummary[];
  unpinned: NetworkChannelSummary[];
  pinnedIds: ReadonlyArray<string>;
  isPinned: (channel: string) => boolean;
  togglePinned: (channel: string) => void;
  isLoading: boolean;
  isError: boolean;
  error: Error | null;
}

export interface UseNetworkChannelsOptions {
  enabled?: boolean;
  workspaceId?: string | null;
  recentLimit?: number;
}

export function useNetworkChannels({
  enabled: enabledOption = true,
  workspaceId: requestedWorkspaceId,
  recentLimit = NETWORK_DEFAULT_RECENTS_LIMIT,
}: UseNetworkChannelsOptions = {}): UseNetworkChannelsResult {
  const { activeWorkspaceId } = useActiveWorkspace();
  const liveDataEnabled = useNetworkLiveDataEnabled();
  const selectedWorkspaceId = requestedWorkspaceId ?? activeWorkspaceId;
  const workspaceId = selectedWorkspaceId ?? "";
  const enabled = enabledOption && liveDataEnabled && Boolean(selectedWorkspaceId);
  const query = useQuery(
    networkChannelsOptions(workspaceId, { recent_limit: recentLimit }, enabled)
  );
  const workspaceKey = selectedWorkspaceId ?? "";
  const { store } = useStoreBinding(workspaceKey, () => createPinnedChannelsStore(workspaceKey));
  const pinnedIds = useSelector(store, snapshot => snapshot.context.pinnedIds);

  useEffect(() => {
    if (typeof window === "undefined") return undefined;
    void rehydratePinnedChannelsStore(store);

    function handleStorage(event: StorageEvent) {
      if (event.key === PINNED_CHANNELS_STORAGE_KEY) {
        void rehydratePinnedChannelsStore(store);
      }
    }
    window.addEventListener("storage", handleStorage);
    return () => window.removeEventListener("storage", handleStorage);
  }, [store]);

  const togglePinned = (channel: string) => {
    if (!selectedWorkspaceId) return;
    store.trigger.pinToggled({
      workspaceId: workspaceKey,
      channel,
    });
  };
  const channels = query.data?.channels ?? [];
  const pinnedIdSet = new Set(pinnedIds);
  const isPinned = (channel: string) => pinnedIdSet.has(channel);
  const pinned = channels.filter(channel => pinnedIdSet.has(channel.channel));
  const unpinned = channels.filter(channel => !pinnedIdSet.has(channel.channel));

  return {
    channels,
    recents: query.data?.recents ?? [],
    pinned,
    unpinned,
    pinnedIds,
    isPinned,
    togglePinned,
    isLoading: query.isLoading,
    isError: query.isError,
    error: query.error ?? null,
  };
}
