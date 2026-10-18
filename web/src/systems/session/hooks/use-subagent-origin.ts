import { useQuery } from "@tanstack/react-query";

import type { SubagentOriginParent } from "../components/subagents/subagent-origin-divider";
import { sessionDetailOptions } from "../lib/query-options";
import { getSessionDisplayTitle } from "../lib/session-display-title";
import { subagentParentSessionId } from "../lib/session-origin";
import type { SessionPayload } from "../types";

export interface SubagentOriginView {
  /** `null` once the parent read settles without one (deleted or not visible). */
  parent: (SubagentOriginParent & { agentName: string }) | null;
}

/**
 * The "Subagent of" divider's parent (S4), read through the canonical detail
 * cache. `null` for sessions that are not delegated subagents, and while the
 * parent read is still in flight (the divider never names a guess).
 */
export function useSubagentOrigin(
  session: SessionPayload,
  workspaceId: string
): SubagentOriginView | null {
  const parentId = subagentParentSessionId(session) ?? "";
  const parentQuery = useQuery({
    ...sessionDetailOptions(workspaceId, parentId),
    enabled: workspaceId !== "" && parentId !== "",
    refetchInterval: false,
    retry: false,
  });
  if (parentId === "") return null;
  if (parentQuery.data) {
    return {
      parent: {
        id: parentId,
        title: getSessionDisplayTitle(parentQuery.data),
        agentName: parentQuery.data.agent_name,
      },
    };
  }
  return parentQuery.isError ? { parent: null } : null;
}
