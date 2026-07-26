import { useQuery } from "@tanstack/react-query";

import { useActiveWorkspace } from "@/systems/workspace";

import { networkChannelDetailOptions } from "../lib/query-options";
import type { NetworkChannel } from "../types";

export interface ActiveNetworkSession {
  channel: string;
  peerId: string;
  sessionId: string;
  displayName: string | undefined;
}

export interface UseActiveNetworkSessionResult {
  session: ActiveNetworkSession | null;
  /** When non-null, the composer should be disabled with this reason. */
  disabledReason: string | null;
  isLoading: boolean;
}

function pickLocalPeer(channel: NetworkChannel | null | undefined) {
  if (!channel) {
    return null;
  }
  const peers = channel.peers ?? [];
  for (const peer of peers) {
    if (peer.local && peer.session_id) {
      return peer;
    }
  }
  for (const peer of peers) {
    if (peer.local) {
      return peer;
    }
  }
  return null;
}

/**
 * Resolves the current operator's session/peer for sending in a channel. The
 * MVP picks the first `local: true` peer from `getNetworkChannel`; the composer
 * is disabled when the channel has no local peer (`_design.md` §7.4).
 */
export function useActiveNetworkSession(
  channel: string | null | undefined,
  options?: { workspaceId?: string | null }
): UseActiveNetworkSessionResult {
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = options?.workspaceId ?? activeWorkspaceId ?? "";
  const enabled = Boolean(channel) && workspaceId !== "";
  const detailQuery = useQuery(networkChannelDetailOptions(workspaceId, channel ?? "", enabled));

  if (!enabled) {
    return {
      session: null,
      disabledReason: "Pick a channel to start composing.",
      isLoading: false,
    };
  }
  if (detailQuery.isLoading && !detailQuery.data) {
    return { session: null, disabledReason: "Loading channel...", isLoading: true };
  }
  const detail = detailQuery.data ?? null;
  const peer = pickLocalPeer(detail);
  if (!peer || !peer.session_id) {
    return {
      session: null,
      disabledReason: "Join this channel from another surface to compose here.",
      isLoading: false,
    };
  }
  return {
    session: {
      channel: detail?.channel ?? channel ?? "",
      peerId: peer.peer_id,
      sessionId: peer.session_id,
      displayName: peer.display_name,
    },
    disabledReason: null,
    isLoading: false,
  };
}
