import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { useActiveWorkspace } from "@/systems/workspace";

import { NETWORK_DEFAULT_RECENTS_LIMIT, networkChannelsOptions } from "../lib/query-options";
import type { NetworkChannelSummary, NetworkChannelsResponse } from "../types";

const PINNED_CHANNELS_STORAGE_KEY = "network:pinned-channels";
type PinnedChannelsState = Record<string, string[]>;

function readPinnedChannelsState(): PinnedChannelsState {
  if (typeof window === "undefined") return {};
  try {
    const parsed = JSON.parse(window.localStorage.getItem(PINNED_CHANNELS_STORAGE_KEY) ?? "{}");
    if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) return {};
    const state: PinnedChannelsState = {};
    for (const [workspaceId, channels] of Object.entries(parsed)) {
      if (typeof workspaceId !== "string" || !Array.isArray(channels)) continue;
      const clean = channels.filter(item => typeof item === "string" && item.trim() !== "");
      if (clean.length > 0) state[workspaceId] = clean;
    }
    return state;
  } catch {
    return {};
  }
}

function readPinnedChannels(workspaceId: string | null | undefined): string[] {
  return workspaceId ? (readPinnedChannelsState()[workspaceId] ?? []) : [];
}

function writePinnedChannels(workspaceId: string, values: string[]): void {
  if (typeof window === "undefined") return;
  try {
    const next = { ...readPinnedChannelsState() };
    if (values.length === 0) delete next[workspaceId];
    else next[workspaceId] = values;
    window.localStorage.setItem(PINNED_CHANNELS_STORAGE_KEY, JSON.stringify(next));
  } catch {
    // localStorage is best-effort; stay quiet on quota or privacy mode failures.
  }
}

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
  const selectedWorkspaceId = requestedWorkspaceId ?? activeWorkspaceId;
  const workspaceId = selectedWorkspaceId ?? "";
  const enabled = enabledOption && Boolean(selectedWorkspaceId);
  const query = useQuery(
    networkChannelsOptions(workspaceId, { recent_limit: recentLimit }, enabled)
  );
  const workspaceKey = selectedWorkspaceId ?? "";
  const [pinnedState, setPinnedState] = useState(() => ({
    ids: readPinnedChannels(selectedWorkspaceId),
    workspaceKey,
  }));
  const pinnedStateRef = useRef(pinnedState);
  const pinnedIds =
    pinnedState.workspaceKey === workspaceKey
      ? pinnedState.ids
      : readPinnedChannels(selectedWorkspaceId);

  useEffect(() => {
    if (typeof window === "undefined") return undefined;
    function handleStorage(event: StorageEvent) {
      if (event.key === PINNED_CHANNELS_STORAGE_KEY) {
        const next = {
          ids: readPinnedChannels(selectedWorkspaceId),
          workspaceKey,
        };
        pinnedStateRef.current = next;
        setPinnedState(next);
      }
    }
    window.addEventListener("storage", handleStorage);
    return () => window.removeEventListener("storage", handleStorage);
  }, [activeWorkspaceId, requestedWorkspaceId, selectedWorkspaceId, workspaceKey]);

  const togglePinned = (channel: string) => {
    if (!selectedWorkspaceId) return;
    const current = pinnedStateRef.current;
    const currentIds = current.workspaceKey === workspaceKey ? current.ids : pinnedIds;
    const nextIds = currentIds.includes(channel)
      ? currentIds.filter(value => value !== channel)
      : [channel, ...currentIds];
    const next = { ids: nextIds, workspaceKey };
    pinnedStateRef.current = next;
    setPinnedState(next);
    writePinnedChannels(selectedWorkspaceId, nextIds);
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

export const PINNED_CHANNELS_STORAGE_KEY_FOR_TESTS = PINNED_CHANNELS_STORAGE_KEY;
