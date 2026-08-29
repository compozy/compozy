import { childStatesForRoot, type AgentCommsScope, type ChildState } from "@/systems/agent-comms";
import { useSessions } from "@/systems/session";

/** One rendered tree, and the children its rows named. */
export interface ActivityRootChildren {
  rootSessionId: string;
  childSessionIds: readonly string[];
}

export function useActivityChildStates(
  scope: AgentCommsScope,
  roots: readonly ActivityRootChildren[],
  live: boolean
): ReadonlyMap<string, ChildState> {
  const childSessionIds = roots.reduce<string[]>((ids, root) => {
    for (const id of root.childSessionIds) {
      if (id !== "") ids.push(id);
    }
    return ids;
  }, []);
  const catalog = useSessions(scope.workspaceId, {
    enabled: live && scope.workspaceId.trim() !== "" && childSessionIds.length > 0,
    loadAll: true,
  });
  return childStatesForRoot(childSessionIds, catalog.data);
}
