import { useChannelMembers, type UseChannelMembersResult } from "./use-channel-members";
import { useInspectorState, type UseInspectorStateResult } from "./use-inspector-state";

export interface UseNetworkInspectorViewArgs {
  workspaceId: string | null | undefined;
  channel: string | null | undefined;
  /**
   * The view should only fetch members while the inspector is actually
   * visible — when it's collapsed or a thread overlay is taking the right
   * rail, the inspector queries stay quiet.
   */
  enabled: boolean;
}

export interface UseNetworkInspectorViewResult {
  inspector: UseInspectorStateResult;
  members: UseChannelMembersResult;
}

/**
 * Composite view-model for the right-rail Network inspector: the per-channel
 * inspector state plus the members feed backing the Members segment.
 */
export function useNetworkInspectorView({
  channel,
  enabled,
  workspaceId,
}: UseNetworkInspectorViewArgs): UseNetworkInspectorViewResult {
  const inspector = useInspectorState(channel);
  const queryEnabled = enabled && inspector.open && Boolean(channel);
  const members = useChannelMembers(channel, { enabled: queryEnabled, workspaceId });

  return { inspector, members };
}
