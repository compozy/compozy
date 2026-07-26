import { useQuery } from "@tanstack/react-query";

import { toolApprovalGrantsListOptions } from "../lib/query-options";

/** Workspace-scoped list query for remembered native-tool approval decisions. */
export function useToolApprovalGrants(workspaceId: string) {
  return useQuery(toolApprovalGrantsListOptions(workspaceId));
}
