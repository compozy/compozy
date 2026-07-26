import { useActiveWorkspaceStore } from "@/systems/workspace/hooks/use-active-workspace-store";

import type { RoutingCoordinator } from "./routing-coordinator";

/** Enters hydration synchronously, before route layout effects can report the new location. */
export function subscribeWorkspaceSwitchBarrier(
  coordinator: Pick<RoutingCoordinator, "beginWorkspaceSwitch">
): () => void {
  return useActiveWorkspaceStore.subscribe((state, previous) => {
    if (state.selectedWorkspaceId !== previous.selectedWorkspaceId) {
      coordinator.beginWorkspaceSwitch();
    }
  });
}
