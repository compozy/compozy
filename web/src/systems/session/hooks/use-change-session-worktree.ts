import { useMutation, useQueryClient } from "@tanstack/react-query";
import { notifyUser } from "@/lib/user-feedback";
import { workspaceKeys } from "@/systems/workspace";
import { changeSessionWorktree } from "../adapters/session-worktree-api";
import { sessionKeys } from "../lib/query-keys";

export function useChangeSessionWorktree(workspaceId: string, sessionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({
      worktree,
      expectedWorktreeId,
    }: {
      worktree: string;
      expectedWorktreeId: string;
    }) => changeSessionWorktree(workspaceId, sessionId, worktree, expectedWorktreeId),
    onError: error => notifyUser({ tone: "error", message: error.message }),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: sessionKeys.detail(workspaceId, sessionId) }),
        queryClient.invalidateQueries({ queryKey: sessionKeys.byIdRoot(sessionId) }),
        queryClient.invalidateQueries({ queryKey: sessionKeys.workspaceLists(workspaceId) }),
        queryClient.invalidateQueries({ queryKey: sessionKeys.commands(workspaceId, sessionId) }),
        queryClient.invalidateQueries({ queryKey: workspaceKeys.worktrees(workspaceId) }),
      ]);
    },
  });
}
