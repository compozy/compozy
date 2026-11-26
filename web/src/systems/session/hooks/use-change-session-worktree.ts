import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useCurrentComputerId } from "@/hooks/use-current-computer-id";
import type { ComputerId } from "@/lib/computer-scope";
import { notifyUser } from "@/lib/user-feedback";
import { workspaceKeys } from "@/systems/workspace";
import { changeSessionWorktree } from "../adapters/session-worktree-api";
import { sessionKeys } from "../lib/query-keys";
import { resetSessionRootReads } from "./session-root-change";

export function useChangeSessionWorktree(
  workspaceId: string,
  sessionId: string,
  computerId?: ComputerId
) {
  const queryClient = useQueryClient();
  const scopedComputerId = useCurrentComputerId(computerId);
  return useMutation({
    mutationFn: ({
      worktree,
      expectedWorktreeId,
    }: {
      worktree: string;
      expectedWorktreeId: string;
    }) =>
      changeSessionWorktree(
        workspaceId,
        sessionId,
        worktree,
        expectedWorktreeId,
        undefined,
        scopedComputerId
      ),
    onError: error => notifyUser({ tone: "error", message: error.message }),
    onSuccess: async () => {
      const keys = sessionKeys.forComputer(scopedComputerId);
      const wsKeys = workspaceKeys.forComputer(scopedComputerId);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: keys.detail(workspaceId, sessionId) }),
        queryClient.invalidateQueries({ queryKey: keys.byIdRoot(sessionId) }),
        queryClient.invalidateQueries({ queryKey: keys.workspaceLists(workspaceId) }),
        queryClient.invalidateQueries({ queryKey: keys.commands(workspaceId, sessionId) }),
        queryClient.invalidateQueries({ queryKey: wsKeys.worktrees(workspaceId) }),
        resetSessionRootReads(queryClient, workspaceId, sessionId, () => true, scopedComputerId),
      ]);
    },
  });
}
