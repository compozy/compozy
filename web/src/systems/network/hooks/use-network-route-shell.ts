import { useEffect } from "react";

import type { ChannelTab } from "../components/shell/channel-tabs-types";
import {
  networkThreadsLocation,
  parseNetworkWindowLocation,
  type NetworkWindowLocation,
  type ParsedNetworkWindowLocation,
} from "../lib/network-window-location";
import type { NetworkChannelSummary, NetworkRecentEntry } from "../types";
import { useActiveWorkspace } from "@/systems/workspace";
import { useNetworkPage, type UseNetworkPageResult } from "./use-network-page";

export interface UseNetworkRouteShellArgs {
  active: boolean;
  location: NetworkWindowLocation;
  navigation: NetworkWindowNavigation;
}

export interface NetworkWindowNavigation {
  push: (location: NetworkWindowLocation) => void;
}

export interface NetworkRouteShellResult {
  page: UseNetworkPageResult;
  activeChannel: NetworkChannelSummary | null;
  activeTab: ChannelTab;
  /** Active thread route id when the URL targets `/threads/$threadId`. */
  activeThreadId: string | null;
  /** Active direct-room route id when the URL targets `/directs/$directId`. */
  activeDirectId: string | null;
  activeWorkspaceId: string | null;
  location: ParsedNetworkWindowLocation;
  hasUnread: (channelId: string) => boolean;
}

export function useNetworkRouteShell({
  active,
  location,
  navigation,
}: UseNetworkRouteShellArgs): NetworkRouteShellResult {
  const { activeWorkspaceId, selectedWorkspaceId, setActiveWorkspaceId } = useActiveWorkspace();
  const parsed = parseNetworkWindowLocation(location);
  const routeWorkspaceId = parsed.workspaceId ?? activeWorkspaceId;
  const routeWorkspaceAllowed =
    parsed.workspaceId == null ||
    selectedWorkspaceId == null ||
    selectedWorkspaceId === parsed.workspaceId;
  const page = useNetworkPage(routeWorkspaceId, { enabled: routeWorkspaceAllowed });

  useEffect(() => {
    if (!active || !parsed.workspaceId || parsed.workspaceId === activeWorkspaceId) {
      return;
    }
    if (selectedWorkspaceId !== null && selectedWorkspaceId !== parsed.workspaceId) {
      navigation.push({ pathname: "/network", search: {} });
      return;
    }
    setActiveWorkspaceId(parsed.workspaceId);
  }, [
    active,
    activeWorkspaceId,
    navigation,
    parsed.workspaceId,
    selectedWorkspaceId,
    setActiveWorkspaceId,
  ]);

  useEffect(() => {
    if (!active || (parsed.workspaceId != null && parsed.channel != null)) {
      return;
    }
    const target = page.firstVisibleChannel?.channel;
    if (!target || !routeWorkspaceId) {
      return;
    }
    navigation.push(networkThreadsLocation(routeWorkspaceId, target));
  }, [
    active,
    navigation,
    page.firstVisibleChannel,
    parsed.channel,
    parsed.workspaceId,
    routeWorkspaceId,
  ]);

  const activeChannel =
    parsed.workspaceId == null || parsed.workspaceId === routeWorkspaceId
      ? (page.channels.find(channel => channel.channel === parsed.channel) ?? null)
      : null;
  return {
    page,
    activeChannel,
    activeTab: parsed.activeTab,
    activeThreadId: parsed.activeThreadId,
    activeDirectId: parsed.activeDirectId,
    activeWorkspaceId: routeWorkspaceId,
    location: parsed,
    // A dot means "known unread" from the bounded embedded recents projection.
    hasUnread: (channelId: string): boolean =>
      page.recents.some(recent => recent.channel === channelId && recent.hasUnread),
  };
}

export type { NetworkRecentEntry };
