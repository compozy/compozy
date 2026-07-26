import { useMutation, useQueryClient } from "@tanstack/react-query";

import { revokeToolApprovalGrant, setToolApprovalGrant } from "../adapters/tool-approvals-api";
import { toolApprovalKeys } from "../lib/query-keys";
import type { ToolApprovalGrantSetRequest } from "../types";

export interface SetToolApprovalGrantParams {
  workspaceId: string;
  request: ToolApprovalGrantSetRequest;
}

export interface RevokeToolApprovalGrantParams {
  id: string;
  workspaceId: string;
}

/** Set one wider decision, then reread only its workspace's daemon-owned list. */
export function useSetToolApprovalGrant() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceId, request }: SetToolApprovalGrantParams) =>
      setToolApprovalGrant(workspaceId, request),
    onSuccess: (_result, { workspaceId }) =>
      queryClient.invalidateQueries({ queryKey: toolApprovalKeys.list(workspaceId) }),
  });
}

/**
 * Revoke one remembered decision.
 *
 * On success, invalidate **only** the owning workspace's list key so the row
 * disappears against daemon truth (no optimistic removal that could diverge).
 * On error, the cache is left untouched so the row stays visible.
 */
export function useRevokeToolApprovalGrant() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, workspaceId }: RevokeToolApprovalGrantParams) =>
      revokeToolApprovalGrant(id, workspaceId),
    onSuccess: (_result, { workspaceId }) =>
      queryClient.invalidateQueries({ queryKey: toolApprovalKeys.list(workspaceId) }),
  });
}
