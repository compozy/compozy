import { useQuery } from "@tanstack/react-query";

import { useActiveWorkspace } from "@/systems/workspace";

import { toNetworkPresenceState } from "../lib/network-formatters";
import { networkPeersOptions } from "../lib/query-options";
import type { NetworkPeerSummary, NetworkPresenceState } from "../types";

export type ChannelMemberRole = "agent" | "human";

export interface ChannelMember {
  peerId: string;
  sessionId: string;
  displayName: string;
  role: ChannelMemberRole;
  local: boolean;
  presenceState: NetworkPresenceState;
}

export interface UseChannelMembersResult {
  members: ChannelMember[];
  agentCount: number;
  humanCount: number;
  isLoading: boolean;
  error: Error | null;
}

/**
 * Classify a peer summary into AGENT vs HUMAN. The runtime currently does not
 * persist a `kind` field on `NetworkPeerPayload`, so we treat the presence of a
 * local agent session (`session_id`) as the AGENT signal , peers without a
 * session id are assumed to be humans (operators acting against the AGH
 * Network). When the daemon eventually exposes an explicit kind, swap this
 * heuristic for the canonical field.
 */
function classifyPeer(peer: NetworkPeerSummary): ChannelMemberRole {
  return peer.session_id != null && peer.session_id !== "" ? "agent" : "human";
}

export function useChannelMembers(
  channel: string | null | undefined,
  options?: { enabled?: boolean; workspaceId?: string | null }
): UseChannelMembersResult {
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = options?.workspaceId ?? activeWorkspaceId ?? "";
  const enabled = (options?.enabled ?? true) && Boolean(channel) && workspaceId !== "";
  const query = useQuery(networkPeersOptions(workspaceId, channel ?? undefined, enabled));

  const peers: ReadonlyArray<NetworkPeerSummary> = query.data ?? [];
  const members: ChannelMember[] = peers.map((peer: NetworkPeerSummary) => ({
    peerId: peer.peer_id,
    sessionId: peer.session_id ?? "",
    displayName: (peer.display_name?.trim() || peer.peer_card?.display_name?.trim()) ?? "",
    role: classifyPeer(peer),
    local: Boolean(peer.local),
    presenceState: toNetworkPresenceState(peer.presence_state),
  }));
  members.sort((left, right) => {
    if (left.role !== right.role) {
      return left.role === "agent" ? -1 : 1;
    }
    return left.peerId.localeCompare(right.peerId);
  });
  let agentCount = 0;
  let humanCount = 0;
  for (const member of members) {
    if (member.role === "agent") agentCount += 1;
    else humanCount += 1;
  }
  return {
    members,
    agentCount,
    humanCount,
    isLoading: enabled && query.isLoading,
    error: (query.error as Error | null) ?? null,
  };
}
