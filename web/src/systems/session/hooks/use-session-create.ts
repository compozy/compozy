import { use } from "react";
import { useSelector } from "@xstate/store-react";
import { toast } from "sonner";

import { SessionCreateContext } from "../contexts/session-create-context-value";
import type { SessionCreateStore } from "../stores/session-create-store";
import { useActiveWorkspace } from "@/systems/workspace";

export function useSessionCreateStore(): SessionCreateStore {
  const store = use(SessionCreateContext);
  if (!store) {
    throw new Error("useSessionCreateStore must be used inside <SessionCreateProvider>");
  }
  return store;
}

export function useSessionCreateActions() {
  const store = useSessionCreateStore();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const openForAgent = (agentName: string) => {
    if (runtimeWorkspaceId === null) {
      toast.error("Select an active workspace before starting a session.");
      return;
    }
    store.trigger.dialogOpened({ agentName, workspaceId: runtimeWorkspaceId });
  };

  /**
   * Starts session creation already pointed at one worktree, so the operator is
   * never asked to re-pick the environment they just came from.
   */
  const openForWorktree = (workspaceId: string, worktreeId: string) => {
    if (workspaceId.trim() === "") {
      toast.error("Choose a workspace before starting a session.");
      return;
    }
    store.trigger.dialogOpened({
      agentName: "",
      workspaceId,
      environment: { kind: "worktree", worktreeId },
    });
  };

  return { openForAgent, openForWorktree };
}

export function useSessionCreateHasActiveWorkspace(): boolean {
  return useActiveWorkspace().runtimeWorkspaceId !== null;
}

export function useSessionCreateIsCreating(): boolean {
  const store = useSessionCreateStore();
  return useSelector(store, snapshot => snapshot.context.operation.status === "submitting");
}

export function useSessionCreatePendingAgentName(): string | null {
  const store = useSessionCreateStore();
  return useSelector(store, snapshot => {
    const { operation } = snapshot.context;
    return operation.status === "submitting" ? operation.agentName : null;
  });
}
