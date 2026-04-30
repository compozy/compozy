import { useMutation, useQuery, useQueryClient, type QueryKey } from "@tanstack/react-query";

import { dashboardKeys } from "@/systems/dashboard";

import {
  cancelRun,
  getRun,
  getRunSnapshot,
  getRunTranscript,
  listRuns,
  startWorkflowRun,
  type CancelRunParams,
  type StartWorkflowRunParams,
} from "../adapters/runs-api";
import { runKeys } from "../lib/query-keys";
import type { Run, RunListParams, RunSnapshot, RunTranscript } from "../types";

export function useRuns(params: RunListParams) {
  return useQuery<Run[]>({
    queryKey: runKeys.list(params) as QueryKey,
    queryFn: () => listRuns(params),
    enabled: Boolean(params.workspaceId),
    refetchInterval: 3_000,
    refetchIntervalInBackground: false,
  });
}

export function useRun(runId: string | null) {
  return useQuery<Run>({
    queryKey: runKeys.run(runId ?? "none") as QueryKey,
    queryFn: () => {
      if (!runId) {
        throw new Error("run id is required to fetch run summary");
      }
      return getRun(runId);
    },
    enabled: Boolean(runId),
  });
}

export function useRunSnapshot(runId: string | null) {
  return useQuery<RunSnapshot>({
    queryKey: runKeys.snapshot(runId ?? "none") as QueryKey,
    queryFn: () => {
      if (!runId) {
        throw new Error("run id is required to fetch run snapshot");
      }
      return getRunSnapshot(runId);
    },
    enabled: Boolean(runId),
  });
}

export function useRunTranscript(runId: string | null) {
  return useQuery<RunTranscript>({
    queryKey: runKeys.transcript(runId ?? "none") as QueryKey,
    queryFn: () => {
      if (!runId) {
        throw new Error("run id is required to fetch run transcript");
      }
      return getRunTranscript(runId);
    },
    enabled: Boolean(runId),
  });
}

export function useCancelRun() {
  const queryClient = useQueryClient();
  return useMutation<void, Error, CancelRunParams>({
    mutationFn: params => cancelRun(params),
    onSuccess: (_result, variables) => {
      void queryClient.invalidateQueries({ queryKey: runKeys.run(variables.runId) as QueryKey });
      void queryClient.invalidateQueries({
        queryKey: runKeys.snapshot(variables.runId) as QueryKey,
      });
      void queryClient.invalidateQueries({
        queryKey: runKeys.transcript(variables.runId) as QueryKey,
      });
      void queryClient.invalidateQueries({ queryKey: runKeys.lists() as QueryKey });
    },
  });
}

export function useStartWorkflowRun() {
  const queryClient = useQueryClient();
  return useMutation<Run, Error, StartWorkflowRunParams>({
    mutationFn: params => startWorkflowRun(params),
    onSuccess: (_result, variables) => {
      void queryClient.invalidateQueries({ queryKey: runKeys.lists() as QueryKey });
      void queryClient.invalidateQueries({
        queryKey: dashboardKeys.byWorkspace(variables.workspaceId),
      });
    },
  });
}
