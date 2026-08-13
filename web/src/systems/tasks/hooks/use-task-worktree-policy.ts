import { useMutation, useQueryClient } from "@tanstack/react-query";

import { setTaskWorktreePolicy } from "../adapters/task-worktree-policy-api";
import { tasksKeys } from "../lib/query-keys";
import { acknowledgeTaskMutationSettlement } from "../lib/task-mutation";
import type { TaskWorktreePolicySetRequest } from "../types";

interface SetWorktreePolicyParams {
  id: string;
  data: TaskWorktreePolicySetRequest;
}

/**
 * The single web write path for a task's worktree policy.
 *
 * It patches only that block, so a stale draft of any other block can never ride
 * along. The response is the freshly merged profile, which seeds the profile
 * cache before the related reads are invalidated.
 */
export function useSetTaskWorktreePolicy() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: SetWorktreePolicyParams) => setTaskWorktreePolicy(id, data),
    onSuccess: (profile, { id }) => {
      queryClient.setQueryData(tasksKeys.profile(id), profile);
    },
    onSettled: (_result, error, { id }) => {
      acknowledgeTaskMutationSettlement(error);
      return Promise.all([
        queryClient.invalidateQueries({ queryKey: tasksKeys.profile(id) }),
        queryClient.invalidateQueries({ queryKey: tasksKeys.detail(id) }),
        queryClient.invalidateQueries({ queryKey: tasksKeys.lists() }),
        queryClient.invalidateQueries({ queryKey: tasksKeys.agentContextsRoot() }),
      ]);
    },
  });
}
