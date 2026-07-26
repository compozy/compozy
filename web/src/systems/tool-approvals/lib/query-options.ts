import { queryOptions } from "@tanstack/react-query";

import { listToolApprovalGrants } from "../adapters/tool-approvals-api";
import { toolApprovalKeys } from "./query-keys";

/**
 * Options factory for the workspace-scoped list read. The query is gated on a
 * non-empty workspace id (the active workspace is `null` until store hydration),
 * and the `queryFn` forwards the query's `AbortSignal` to the GET.
 */
export function toolApprovalGrantsListOptions(workspaceId: string) {
  return queryOptions({
    queryKey: toolApprovalKeys.list(workspaceId),
    queryFn: ({ signal }) => listToolApprovalGrants(workspaceId, signal),
    enabled: !!workspaceId,
    staleTime: 30_000,
  });
}
