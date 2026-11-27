import { useMutation, useQueryClient } from "@tanstack/react-query";

import { useCurrentComputerId } from "@/hooks/use-current-computer-id";
import type { ComputerId } from "@/lib/computer-scope";

import { forkSessionToWorktree } from "../adapters/session-worktree-api";
import { sessionKeys } from "../lib/query-keys";
import type { ForkSessionToWorktreeParams } from "../types";

/**
 * Forks a live session into a worktree.
 *
 * The result is a second session; the origin is untouched, so only the session
 * lists need rereading. Each confirmation produces exactly one fork — the
 * mutation is never fired speculatively.
 */
export function useForkSessionToWorktree(
  workspaceId: string,
  sessionId: string,
  computerId?: ComputerId
) {
  const queryClient = useQueryClient();
  const scopedComputerId = useCurrentComputerId(computerId);

  return useMutation({
    mutationFn: (params: ForkSessionToWorktreeParams) =>
      forkSessionToWorktree(workspaceId, sessionId, params, undefined, scopedComputerId),
    onSuccess: () =>
      queryClient.invalidateQueries({
        queryKey: sessionKeys.forComputer(scopedComputerId).lists(),
      }),
  });
}
