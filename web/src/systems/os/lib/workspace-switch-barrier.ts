import type { RoutingCoordinator } from "./routing-coordinator";
import { activeWorkspaceStore, desktopPartitionFor } from "@/systems/workspace";

function requestedPartition(): string {
  const context = activeWorkspaceStore.getSnapshot().context;
  return desktopPartitionFor(context.breadthMode, context.focus);
}

/**
 * Enters hydration synchronously, before route layout effects can report the
 * new location. Keys on the destination's partition, so project ↔ User is a
 * switch even though no project id changes.
 */
export function subscribeWorkspaceSwitchBarrier(
  coordinator: Pick<RoutingCoordinator, "beginWorkspaceSwitch">,
  currentWorkspaceId: () => string | null = () => null
): () => void {
  let previousPartition = requestedPartition();
  return activeWorkspaceStore.subscribe(() => {
    const nextPartition = requestedPartition();
    if (nextPartition === previousPartition) return;

    previousPartition = nextPartition;
    // A destination change that keeps the bound partition (All projects with a
    // project focused) has no runtime rebind to follow; entering hydration
    // would strand route reconciliation.
    if (nextPartition === currentWorkspaceId()) return;
    coordinator.beginWorkspaceSwitch();
  }).unsubscribe;
}
