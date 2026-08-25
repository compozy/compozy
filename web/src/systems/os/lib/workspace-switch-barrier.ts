import type { RoutingCoordinator } from "./routing-coordinator";
import { activeWorkspaceStore } from "@/systems/workspace";

/** Enters hydration synchronously, before route layout effects can report the new location. */
export function subscribeWorkspaceSwitchBarrier(
  coordinator: Pick<RoutingCoordinator, "beginWorkspaceSwitch">,
  currentWorkspaceId: () => string | null = () => null
): () => void {
  let previousWorkspaceId = activeWorkspaceStore.getSnapshot().context.selectedWorkspaceId;
  return activeWorkspaceStore.subscribe(snapshot => {
    const nextWorkspaceId = snapshot.context.selectedWorkspaceId;
    if (nextWorkspaceId === previousWorkspaceId) return;

    previousWorkspaceId = nextWorkspaceId;
    // Global mode can acquire a remembered project without changing the
    // desktop's durable layout partition. Entering hydration in that case
    // would strand route reconciliation because no runtime rebind follows.
    if (nextWorkspaceId !== null && nextWorkspaceId === currentWorkspaceId()) return;
    coordinator.beginWorkspaceSwitch();
  }).unsubscribe;
}
