import { useActiveNetworkSession, type UseActiveNetworkSessionResult } from "./use-active-session";
import { useNetworkDirects, type UseNetworkDirectsResult } from "./use-directs";

export interface UseNetworkRailViewArgs {
  workspaceId: string | null | undefined;
  channel: string | null | undefined;
}

export interface UseNetworkRailViewResult {
  directs: UseNetworkDirectsResult;
  session: UseActiveNetworkSessionResult;
}

/**
 * Composite view-model used by the network route shell to drive the
 * channel rail's `Direct Rooms` section. Bundles the active-channel
 * directs query and the active session lookup so the route component stays
 * under the `compozy-react(max-component-complexity)` hook budget.
 */
export function useNetworkRailView({
  workspaceId,
  channel,
}: UseNetworkRailViewArgs): UseNetworkRailViewResult {
  const directs = useNetworkDirects(channel, { workspaceId });
  const session = useActiveNetworkSession(channel ?? "", { workspaceId });

  return { directs, session };
}
