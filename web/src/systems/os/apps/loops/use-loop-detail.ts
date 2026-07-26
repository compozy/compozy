import { useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";

import {
  LoopsApiError,
  readLoopGraph,
  useCreateLoop,
  useDeleteLoop,
  useLoop,
  useLoopConfig,
  useLoopRuns,
  useLoops,
} from "@/systems/loops";
import { useActiveWorkspace } from "@/systems/workspace";

import { useLoopBindings } from "@/hooks/routes/use-loop-bindings";

const RECENT_RUNS_LIMIT = 5;

/** View-model for the Loop detail route: definition, 30d aggregate, recent runs, bindings, nav. */
export function useLoopDetail(name: string) {
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = activeWorkspaceId ?? "";
  const navigate = useNavigate();
  const active = workspaceId !== "";

  const loopQuery = useLoop(workspaceId, name, active);
  const configQuery = useLoopConfig(workspaceId, name, active);
  const catalogQuery = useLoops(workspaceId, { limit: 50, q: name, sort: "name" }, active);
  const runsQuery = useLoopRuns(workspaceId, { loop: name, limit: RECENT_RUNS_LIMIT }, active);
  const createLoop = useCreateLoop();
  const deleteLoop = useDeleteLoop();
  const bindings = useLoopBindings(active ? workspaceId : "", active ? name : "");

  const catalogEntry = catalogQuery.loops.find(entry => entry.name === name) ?? null;

  const handlers = {
    onRun: () => void navigate({ to: "/loops/$name/run", params: { name } }),
    onConfigure: () => void navigate({ to: "/loops/$name/configure", params: { name } }),
    onOpenEditor: async () => {
      if (workspaceId === "" || createLoop.isPending) return;
      if (loopQuery.data?.source !== "workspace") {
        try {
          await createLoop.mutateAsync({ workspaceId, data: { fork_from_name: name } });
        } catch (error) {
          if (!(error instanceof LoopsApiError && error.status === 409)) {
            toast.error(error instanceof Error ? error.message : "Failed to fork loop.");
            return;
          }
        }
      }
      void navigate({ to: "/loops/$name/editor", params: { name } });
    },
    onDelete: async () => {
      if (workspaceId === "" || loopQuery.data?.source !== "workspace" || deleteLoop.isPending) {
        return;
      }
      try {
        await deleteLoop.mutateAsync({ workspaceId, name });
        toast.success(`Deleted workspace loop ${name}`);
        void navigate({ to: "/loops", replace: true });
      } catch {
        // The mutation retains the primary error for the confirmation dialog.
      }
    },
    onDeleteReset: deleteLoop.reset,
    onAddTrigger: () => void navigate({ to: "/triggers", search: { create: "loop", loop: name } }),
    onAddSchedule: () => void navigate({ to: "/jobs", search: { create: "loop", loop: name } }),
  };

  return {
    workspaceId,
    loopQuery,
    configQuery,
    catalogEntry,
    runsQuery,
    bindings,
    deleteLoop,
    readGraph: readLoopGraph,
    handlers,
  };
}
