import type { QueryClient } from "@tanstack/react-query";

import type { NetworkChannel } from "../types";
import { networkKeys } from "../lib/query-keys";

/** The owner fields an optimistic network message must carry. */
export interface NetworkMessageOwner {
  id: string;
  name: string;
}

/**
 * Resolve the sender's owner from the channel read that selected the session.
 *
 * The profile lens describes what the operator is browsing, while the daemon
 * binds a send to the supplied session. The channel payload is therefore the
 * authoritative client-side owner for an optimistic row; the lens is only a
 * fallback while that payload is still loading.
 */
export function resolveNetworkMessageOwner(
  queryClient: QueryClient,
  workspaceId: string,
  channel: string,
  fallback: NetworkMessageOwner
): NetworkMessageOwner | null {
  const detail = queryClient.getQueryData<NetworkChannel>(
    networkKeys.channelDetail(workspaceId, channel)
  );
  if (detail?.profile_id && detail.profile_name) {
    return { id: detail.profile_id, name: detail.profile_name };
  }
  return fallback.id && fallback.name ? fallback : null;
}
