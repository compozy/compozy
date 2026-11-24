import { useCallback, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { toast } from "@compozy/ui";

import { apiClient, apiRequestFailed, LOCAL_COMPUTER_ID } from "@/lib/api-client";
import { computerQueryKey } from "@/lib/computer-scope";

import { patchComputer, patchLocalComputer } from "../adapters/computers-api";
import { computerRegistryKeys } from "../lib/query-keys";
import type { LoadBalancingWeight } from "../mocks/remote-computers-types";
import type { ComputerListResponse, PatchLocalComputerRequest } from "../types";

async function fetchActiveLocalSessionCount(signal?: AbortSignal): Promise<number> {
  const { data, error, response } = await apiClient.GET("/api/sessions", {
    params: {
      query: {
        all_workspaces: true,
        all_profiles: true,
        state: "active",
        limit: 1,
      },
    },
    signal,
  });
  if (apiRequestFailed(response, error) || !data) {
    return 0;
  }
  return data.page.total ?? data.sessions.length;
}

/**
 * Owns `PATCH /api/computers/local` (`run_agents`, `load_balancing`, `load_balancing_weight`),
 * per-computer `PATCH /api/computers/{id}` weight updates, and the active local session count
 * driving the "Stop running agents on this computer?" confirmation dialog (US-014, US-015).
 */
export function useLocalComputerSettings(options: { signedIn: boolean; localRunAgents: boolean }) {
  const queryClient = useQueryClient();
  const [disableAgentsOpen, setDisableAgentsOpen] = useState(false);

  const activeSessionsQuery = useQuery({
    queryKey: computerQueryKey(LOCAL_COMPUTER_ID, "sessions", "active-count"),
    queryFn: ({ signal }) => fetchActiveLocalSessionCount(signal),
    enabled: options.signedIn && options.localRunAgents,
    staleTime: 10_000,
    retry: false,
  });

  const activeLocalSessions = activeSessionsQuery.data ?? 0;

  const patchLocalMutation = useMutation({
    mutationFn: (body: PatchLocalComputerRequest) => patchLocalComputer(body),
    onSuccess: (updated, vars) => {
      queryClient.setQueryData<ComputerListResponse>(computerRegistryKeys.list(), prev =>
        prev ? { ...prev, local: updated } : prev
      );
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
      if (vars.run_agents !== undefined) {
        toast.message(
          vars.run_agents
            ? "This computer can now run agents."
            : "Stopped starting new agents on this computer."
        );
      } else if (vars.load_balancing !== undefined) {
        toast.message(
          vars.load_balancing ? "Load balancing turned on." : "Load balancing turned off."
        );
      }
    },
    onError: err => {
      toast.error(err instanceof Error ? err.message : "Failed to update local computer settings.");
    },
  });

  const computerWeightMutation = useMutation({
    mutationFn: ({ id, weight }: { id: string; weight: LoadBalancingWeight }) =>
      patchComputer(id, { load_balancing_weight: weight }),
    onSuccess: updated => {
      queryClient.setQueryData<ComputerListResponse>(computerRegistryKeys.list(), prev =>
        prev
          ? {
              ...prev,
              computers: prev.computers.map(item => (item.id === updated.id ? updated : item)),
            }
          : prev
      );
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
    },
    onError: err => {
      toast.error(
        err instanceof Error ? err.message : "Failed to update load balancing preference."
      );
    },
  });

  const handleRequestToggleRunAgents = useCallback(
    (next: boolean) => {
      if (!next && activeLocalSessions > 0) {
        setDisableAgentsOpen(true);
        return;
      }
      patchLocalMutation.mutate({ run_agents: next });
    },
    [activeLocalSessions, patchLocalMutation]
  );

  const handleConfirmDisableAgents = useCallback(() => {
    patchLocalMutation.mutate(
      { run_agents: false },
      {
        onSettled: () => setDisableAgentsOpen(false),
      }
    );
  }, [patchLocalMutation]);

  const handleToggleLoadBalancing = useCallback(
    (next: boolean) => {
      patchLocalMutation.mutate({ load_balancing: next });
    },
    [patchLocalMutation]
  );

  const handleChangeLocalWeight = useCallback(
    (weight: LoadBalancingWeight) => {
      patchLocalMutation.mutate({ load_balancing_weight: weight });
    },
    [patchLocalMutation]
  );

  const handleChangeComputerWeight = useCallback(
    (computerId: string, weight: LoadBalancingWeight) => {
      computerWeightMutation.mutate({ id: computerId, weight });
    },
    [computerWeightMutation]
  );

  return {
    activeLocalSessions,
    disableAgentsOpen,
    setDisableAgentsOpen,
    isUpdatingLocalSettings: patchLocalMutation.isPending,
    isUpdatingLoadBalancing: patchLocalMutation.isPending || computerWeightMutation.isPending,
    handleRequestToggleRunAgents,
    handleConfirmDisableAgents,
    handleToggleLoadBalancing,
    handleChangeLocalWeight,
    handleChangeComputerWeight,
  };
}
