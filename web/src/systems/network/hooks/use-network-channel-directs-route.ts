import { useActiveNetworkSession, type UseActiveNetworkSessionResult } from "./use-active-session";
import { useChannelMembers, type UseChannelMembersResult } from "./use-channel-members";

export interface UseNetworkChannelDirectsRouteResult {
  session: UseActiveNetworkSessionResult;
  members: UseChannelMembersResult;
}

export function useNetworkChannelDirectsRoute(
  workspaceId: string,
  channel: string
): UseNetworkChannelDirectsRouteResult {
  const session = useActiveNetworkSession(channel, { workspaceId });
  const members = useChannelMembers(channel, { workspaceId });
  return { session, members };
}
