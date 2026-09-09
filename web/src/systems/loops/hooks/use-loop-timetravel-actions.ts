import { useLoopRunOwner } from "./use-loop-run-owner";
import { useProfileReadScope } from "@/systems/profiles";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { forkLoopRun, rerunLoopRun } from "../adapters/loops-api";
import { loopsKeys } from "../lib/query-keys";
import { loopRunDiffOptions } from "../lib/query-options";
import type { LoopDiffQuery, LoopForkRequest, LoopRerunRequest } from "../types";

type QueryClient = ReturnType<typeof useQueryClient>;

interface LoopRunTimetravelParams {
  workspaceId: string;
  runId: string;
}

interface RerunParams extends LoopRunTimetravelParams {
  data: LoopRerunRequest;
}

interface ForkParams extends LoopRunTimetravelParams {
  data: LoopForkRequest;
}

function invalidateTimetravel(
  queryClient: QueryClient,
  workspaceId: string,
  runIds: readonly string[]
): Promise<unknown> {
  const pending = [
    queryClient.invalidateQueries({ queryKey: loopsKeys.catalogByWorkspace(workspaceId) }),
    queryClient.invalidateQueries({ queryKey: loopsKeys.runsByWorkspace(workspaceId) }),
    queryClient.invalidateQueries({ queryKey: loopsKeys.nodeInventoryByWorkspace(workspaceId) }),
    queryClient.invalidateQueries({ queryKey: loopsKeys.runDiffRoot() }),
  ];
  for (const runId of runIds) {
    if (!runId) continue;
    pending.push(
      queryClient.invalidateQueries({ queryKey: loopsKeys.runDetail(workspaceId, runId) })
    );
  }
  return Promise.all(pending);
}

export function useLoopRunDiff(
  workspaceId: string,
  runId: string,
  query: LoopDiffQuery = {},
  enabled = true
) {
  const { params } = useProfileReadScope();
  return useQuery(loopRunDiffOptions(workspaceId, runId, query, enabled, params));
}

export function useRerunLoopRun() {
  const queryClient = useQueryClient();
  const resolveOwner = useLoopRunOwner();
  return useMutation({
    mutationFn: async ({ workspaceId, runId, data }: RerunParams) =>
      rerunLoopRun({ workspaceId, runId }, data, undefined, await resolveOwner(workspaceId, runId)),
    onSettled: (_result, _error, { workspaceId, runId }) =>
      invalidateTimetravel(queryClient, workspaceId, [runId]),
  });
}

export function useForkLoopRun() {
  const queryClient = useQueryClient();
  const resolveOwner = useLoopRunOwner();
  return useMutation({
    mutationFn: async ({ workspaceId, runId, data }: ForkParams) =>
      forkLoopRun({ workspaceId, runId }, data, undefined, await resolveOwner(workspaceId, runId)),
    onSettled: (result, _error, { workspaceId, runId }) =>
      invalidateTimetravel(queryClient, workspaceId, [runId, result?.run.id ?? ""]),
  });
}
