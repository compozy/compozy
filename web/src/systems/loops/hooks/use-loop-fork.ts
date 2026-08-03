import { notifyUser } from "@/lib/user-feedback";

import { LoopsApiError } from "../adapters/loops-api";
import { useCreateLoop } from "./use-loop-actions";

export interface UseLoopForkResult {
  fork: () => void;
  forking: boolean;
}

/**
 * Fork-before-publish for a read-only definition (`marketplace`, `user`, or `additional`),
 * reusing the shipped contract from the Loop detail route
 * (`systems/os/apps/loops/use-loop-detail.ts`): `POST /loops` with `fork_from_name` under the
 * SAME name.
 *
 * Because the name does not change, the editor stays on `/loops/$name/editor`. The mutation's
 * `onSettled` invalidates the workspace-scoped detail, so `loop.source` flips to `workspace` on
 * the refetch and `useLoopEditor` re-seeds its draft from the now-editable definition. A 409
 * means a workspace copy already exists — the same tolerated case the detail route allows, and
 * the invalidation still runs, so the editor lands on the existing copy.
 */
export function useLoopFork(workspaceId: string, name: string): UseLoopForkResult {
  const createLoop = useCreateLoop();
  const fork = () => {
    if (workspaceId === "" || name === "" || createLoop.isPending) return;
    void createLoop.mutateAsync({ workspaceId, data: { fork_from_name: name } }).catch(error => {
      if (error instanceof LoopsApiError && error.status === 409) return;
      void notifyUser({
        message: error instanceof Error ? error.message : "Failed to fork loop.",
        tone: "error",
      });
    });
  };
  return { fork, forking: createLoop.isPending };
}
