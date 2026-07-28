import { use } from "react";
import { useSelector } from "@xstate/store-react";
import { toast } from "sonner";

import { useActiveWorkspace } from "@/systems/workspace";
import { SessionCreateContext } from "../contexts/session-create-context-value";
import type { SessionCreateStore } from "../stores/session-create-store";

export function useSessionCreateStore(): SessionCreateStore {
  const store = use(SessionCreateContext);
  if (!store) {
    throw new Error("useSessionCreateStore must be used inside <SessionCreateProvider>");
  }
  return store;
}

export function useSessionCreateActions() {
  const store = useSessionCreateStore();
  const { activeWorkspaceId } = useActiveWorkspace();
  const openForAgent = (agentName: string) => {
    if (activeWorkspaceId === null) {
      toast.error("Select an active workspace before starting a session.");
      return;
    }
    store.trigger.dialogOpened({ agentName, workspaceId: activeWorkspaceId });
  };

  return { openForAgent };
}

export function useSessionCreateHasActiveWorkspace(): boolean {
  return useActiveWorkspace().activeWorkspaceId !== null;
}

export function useSessionCreateIsCreating(): boolean {
  const store = useSessionCreateStore();
  return useSelector(store, snapshot => snapshot.context.isSubmitting);
}

export function useSessionCreatePendingAgentName(): string | null {
  const store = useSessionCreateStore();
  return useSelector(store, snapshot => snapshot.context.pendingAgentName);
}
