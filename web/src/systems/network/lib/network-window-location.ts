import type { ChannelTab } from "../components/shell/channel-tabs-types";
import type { NetworkDirectRoomDetail, NetworkThreadDetail } from "../types";

export interface NetworkWindowLocation {
  pathname: string;
  search: Record<string, unknown>;
}

export interface ParsedNetworkWindowLocation {
  workspaceId: string | null;
  channel: string | null;
  activeTab: ChannelTab;
  activeThreadId: string | null;
  activeDirectId: string | null;
  view: "full" | undefined;
}

function decodePathSegment(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

export function parseNetworkWindowLocation(
  location: NetworkWindowLocation
): ParsedNetworkWindowLocation {
  const match = /^\/network\/([^/]+)\/([^/]+)\/(threads|directs|activity)(?:\/([^/]+))?$/.exec(
    location.pathname
  );
  if (!match) {
    return {
      workspaceId: null,
      channel: null,
      activeTab: "threads",
      activeThreadId: null,
      activeDirectId: null,
      view: undefined,
    };
  }

  const workspaceId = decodePathSegment(match[1]);
  const channel = decodePathSegment(match[2]);
  const activeTab = match[3] as ChannelTab;
  const detailId = match[4] ? decodePathSegment(match[4]) : null;
  return {
    workspaceId,
    channel,
    activeTab,
    activeThreadId: activeTab === "threads" ? detailId : null,
    activeDirectId: activeTab === "directs" ? detailId : null,
    view: activeTab === "threads" && location.search.view === "full" ? "full" : undefined,
  };
}

export function networkThreadsLocation(
  workspaceId: string,
  channel: string,
  threadId?: string,
  view?: "full"
): NetworkWindowLocation {
  const base = networkChannelLocation(workspaceId, channel, "threads").pathname;
  return {
    pathname: threadId ? `${base}/${encodeURIComponent(threadId)}` : base,
    search: view ? { view } : {},
  };
}

export function networkChannelLocation(
  workspaceId: string,
  channel: string,
  activeTab: ChannelTab
): NetworkWindowLocation {
  return {
    pathname: `/network/${encodeURIComponent(workspaceId)}/${encodeURIComponent(channel)}/${activeTab}`,
    search: {},
  };
}

export function getOtherDirectSessionId(
  direct: Pick<NetworkDirectRoomDetail, "session_a" | "session_b">,
  selfSessionId: string | null | undefined
): string | null {
  if (!selfSessionId) return null;
  if (direct.session_a === selfSessionId) return direct.session_b;
  if (direct.session_b === selfSessionId) return direct.session_a;
  return null;
}

export function resolveNetworkConversationLabel(
  location: ParsedNetworkWindowLocation,
  detail: {
    thread: NetworkThreadDetail | null;
    direct: NetworkDirectRoomDetail | null;
    selfSessionId: string | null;
  }
): string | null {
  if (location.activeThreadId) {
    if (detail.thread?.thread_id !== location.activeThreadId) return null;
    return detail.thread.title?.trim() || null;
  }
  if (location.activeDirectId) {
    if (detail.direct?.direct_id !== location.activeDirectId) return null;
    const otherSessionId = getOtherDirectSessionId(detail.direct, detail.selfSessionId);
    return otherSessionId ? `@${otherSessionId}` : null;
  }
  return null;
}

export interface NetworkWindowTrail {
  /** Parent crumbs for the drill-in head (empty at the root). */
  parents: ReadonlyArray<{
    id: "root" | "channel";
    label: string;
    location: NetworkWindowLocation;
  }>;
  /** Leaf title (window H1). */
  leaf: string;
  /** Whether the head shows the back affordance (any drilled-in level). */
  drilledIn: boolean;
}

/**
 * Drill-in trail for the unified 44px head (os/pagehead contract §03).
 * Selecting a channel is rail navigation, not a drill — the head stays at the
 * root level (`Network` + channel count). The trail only appears once a
 * conversation (thread or direct) is open: Network / #channel / <leaf>.
 */
export function networkWindowTrail(
  location: ParsedNetworkWindowLocation,
  conversationLabel?: string | null
): NetworkWindowTrail {
  const inConversation = Boolean(location.activeThreadId || location.activeDirectId);
  if (!location.workspaceId || !location.channel || !inConversation) {
    return { parents: [], leaf: "Network", drilledIn: false };
  }
  return {
    parents: [
      { id: "root", label: "Network", location: { pathname: "/network", search: {} } },
      {
        id: "channel",
        label: `#${location.channel}`,
        location: networkChannelLocation(
          location.workspaceId,
          location.channel,
          location.activeTab
        ),
      },
    ],
    leaf: conversationLabel ?? (location.activeThreadId ? "Thread" : "Direct"),
    drilledIn: true,
  };
}
