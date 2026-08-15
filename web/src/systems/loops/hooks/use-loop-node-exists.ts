import { useQuery } from "@tanstack/react-query";

import { loopNodeExistsOptions } from "../lib/query-options";

/**
 * Workspace-scoped existence probe for one inventory state. True only when the
 * daemon returns at least one item; loading and errors stay false so absence
 * is the signal.
 */
export function useLoopNodeExists(
  workspaceId: string,
  state: "waiting" | "attention",
  enabled = true
): boolean {
  const query = useQuery(loopNodeExistsOptions(workspaceId, state, enabled));
  return (query.data?.items.length ?? 0) > 0;
}
