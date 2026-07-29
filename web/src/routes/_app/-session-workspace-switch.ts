import type { SessionOwnerDialogState } from "@/systems/session";
import { setActiveWorkspaceId } from "@/systems/workspace";

/**
 * Confirming a foreign-workspace deep link delegates the switch to the active-workspace store.
 * The store subscription owns the OS-shell barrier (`beginWorkspaceSwitch`), so the barrier is
 * never called here; re-entering the deep link afterwards is the caller's route-specific step.
 */
export function confirmSessionWorkspaceSwitch(
  owner: SessionOwnerDialogState,
  reenterDeepLink: () => void
): void {
  setActiveWorkspaceId(owner.workspaceId);
  reenterDeepLink();
}
