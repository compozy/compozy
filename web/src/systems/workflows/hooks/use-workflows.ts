import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { dashboardKeys } from "@/systems/dashboard";

import {
  archiveWorkflow,
  listWorkflows,
  syncWorkflow,
  type ArchiveWorkflowParams,
  type SyncWorkflowParams,
} from "../adapters/workflows-api";
import { workflowKeys } from "../lib/query-keys";
import type { ArchiveResult, SyncResult, WorkflowSummary } from "../types";

export function useWorkflows(workspaceId: string | null) {
  return useQuery<WorkflowSummary[]>({
    queryKey: workflowKeys.list(workspaceId ?? "none"),
    queryFn: () => {
      if (!workspaceId) {
        throw new Error("active workspace is required to load workflows");
      }
      return listWorkflows(workspaceId);
    },
    enabled: Boolean(workspaceId),
  });
}

export function useSyncWorkflows() {
  const queryClient = useQueryClient();
  return useMutation<SyncResult, Error, SyncWorkflowParams>({
    mutationFn: params => syncWorkflow(params),
    onSuccess: (_result, variables) => {
      void queryClient.invalidateQueries({
        queryKey: workflowKeys.list(variables.workspaceId),
      });
      void queryClient.invalidateQueries({
        queryKey: dashboardKeys.byWorkspace(variables.workspaceId),
      });
    },
  });
}

export function useArchiveWorkflow() {
  const queryClient = useQueryClient();
  return useMutation<ArchiveResult, Error, ArchiveWorkflowParams>({
    mutationFn: params => archiveWorkflow(params),
    onSuccess: (_result, variables) => {
      void queryClient.invalidateQueries({
        queryKey: workflowKeys.list(variables.workspaceId),
      });
      void queryClient.invalidateQueries({
        queryKey: dashboardKeys.byWorkspace(variables.workspaceId),
      });
    },
  });
}
