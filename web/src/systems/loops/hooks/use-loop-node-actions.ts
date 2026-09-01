import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  cancelLoopNode,
  pauseLoopNode,
  requeueLoopNode,
  resumeLoopNode,
} from "../adapters/loops-api";
import { loopsKeys } from "../lib/query-keys";
import type {
  LoopNodeMutationRequest,
  LoopNodePauseRequest,
  LoopNodeResumeRequest,
} from "../types";

/**
 * Node lifecycle mutations (US-019 / US-024). Every verb reconciles through the
 * canonical keys the owner rereads: the run detail (which carries
 * `node_controls[]` and `waits[]`), the workspace runs lists, and every node
 * inventory view — a pause moves a node out of `retrying` and into no view at
 * all, so a partial invalidation would leave a stale row behind.
 *
 * No optimistic update: the daemon owns the transition, and it can legitimately
 * answer "already in that state" or refuse outright. Painting the new state
 * before it commits is exactly the untruthful UI SD-007 forbids.
 */

type QueryClient = ReturnType<typeof useQueryClient>;

interface LoopNodeParams {
  workspaceId: string;
  runId: string;
  nodeId: string;
}

interface PauseNodeParams extends LoopNodeParams {
  data: LoopNodePauseRequest;
}

interface ResumeNodeParams extends LoopNodeParams {
  data: LoopNodeResumeRequest;
}

interface NodeMutationParams extends LoopNodeParams {
  data?: LoopNodeMutationRequest;
}

function invalidateNodeLifecycle(
  queryClient: QueryClient,
  workspaceId: string,
  runId: string
): Promise<unknown> {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: loopsKeys.runDetail(workspaceId, runId) }),
    queryClient.invalidateQueries({ queryKey: loopsKeys.runsByWorkspace(workspaceId) }),
    queryClient.invalidateQueries({
      queryKey: loopsKeys.nodeInventoryByWorkspace(workspaceId),
    }),
  ]);
}

export function usePauseLoopNode() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceId, runId, nodeId, data }: PauseNodeParams) =>
      pauseLoopNode({ workspaceId, runId, nodeId }, data),
    onSettled: (_result, _error, { workspaceId, runId }) =>
      invalidateNodeLifecycle(queryClient, workspaceId, runId),
  });
}

export function useResumeLoopNode() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceId, runId, nodeId, data }: ResumeNodeParams) =>
      resumeLoopNode({ workspaceId, runId, nodeId }, data),
    onSettled: (_result, _error, { workspaceId, runId }) =>
      invalidateNodeLifecycle(queryClient, workspaceId, runId),
  });
}

export function useCancelLoopNode() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceId, runId, nodeId, data }: NodeMutationParams) =>
      cancelLoopNode({ workspaceId, runId, nodeId }, data),
    onSettled: (_result, _error, { workspaceId, runId }) =>
      invalidateNodeLifecycle(queryClient, workspaceId, runId),
  });
}

export function useRequeueLoopNode() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceId, runId, nodeId, data }: NodeMutationParams) =>
      requeueLoopNode({ workspaceId, runId, nodeId }, data),
    onSettled: (_result, _error, { workspaceId, runId }) =>
      invalidateNodeLifecycle(queryClient, workspaceId, runId),
  });
}
