import type { RoutingCoordinator } from "./routing-coordinator";
import { activeWorkspaceStore } from "@/systems/workspace";

/** Enters hydration synchronously, before route layout effects can report the new location. */
export function subscribeWorkspaceSwitchBarrier(
  coordinator: Pick<RoutingCoordinator, "beginWorkspaceSwitch">
): () => void {
  let previousWorkspaceId = activeWorkspaceStore.getSnapshot().context.selectedWorkspaceId;
  return activeWorkspaceStore.subscribe(snapshot => {
    const nextWorkspaceId = snapshot.context.selectedWorkspaceId;
    if (nextWorkspaceId === previousWorkspaceId) return;

    previousWorkspaceId = nextWorkspaceId;
    coordinator.beginWorkspaceSwitch();
  }).unsubscribe;
}
