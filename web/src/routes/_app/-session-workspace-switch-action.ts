import type { SessionOwnerDialogState } from "@/systems/session";
import { setActiveWorkspaceId } from "@/systems/workspace";

export function confirmSessionWorkspaceSwitch(
  owner: SessionOwnerDialogState,
  reenterDeepLink: () => void
): void {
  setActiveWorkspaceId(owner.workspaceId);
  reenterDeepLink();
}
