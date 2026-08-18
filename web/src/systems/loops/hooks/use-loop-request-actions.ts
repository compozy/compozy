import { useMutation, useQueryClient } from "@tanstack/react-query";

import { amendLoopNode, respondLoopRequest } from "../adapters/loops-api";
import { loopsKeys } from "../lib/query-keys";
import type { LoopAmendRequest, LoopRespondRequest } from "../types";

type QueryClient = ReturnType<typeof useQueryClient>;

interface LoopRequestNodeParams {
  workspaceId: string;
  runId: string;
  nodeId: string;
}

interface RespondParams extends LoopRequestNodeParams {
  data: LoopRespondRequest;
}

interface AmendParams extends LoopRequestNodeParams {
  data: LoopAmendRequest;
}

function invalidateRequestLifecycle(
  queryClient: QueryClient,
  workspaceId: string,
  runId: string
): Promise<unknown> {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: loopsKeys.runDetail(workspaceId, runId) }),
    queryClient.invalidateQueries({ queryKey: loopsKeys.runsByWorkspace(workspaceId) }),
    queryClient.invalidateQueries({ queryKey: loopsKeys.nodeInventoryByWorkspace(workspaceId) }),
    queryClient.invalidateQueries({ queryKey: loopsKeys.requestsByWorkspace(workspaceId) }),
  ]);
}

export function useRespondLoopRequest() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceId, runId, nodeId, data }: RespondParams) =>
      respondLoopRequest({ workspaceId, runId, nodeId }, data),
    onSettled: (_result, _error, { workspaceId, runId }) =>
      invalidateRequestLifecycle(queryClient, workspaceId, runId),
  });
}

export function useAmendLoopNode() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceId, runId, nodeId, data }: AmendParams) =>
      amendLoopNode({ workspaceId, runId, nodeId }, data),
    onSettled: (_result, _error, { workspaceId, runId }) =>
      invalidateRequestLifecycle(queryClient, workspaceId, runId),
  });
}
