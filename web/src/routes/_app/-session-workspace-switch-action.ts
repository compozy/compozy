import type { SessionOwnerDialogState } from "@/systems/session";
import { focusDestination, ownerDataScope } from "@/systems/workspace";

export function confirmSessionWorkspaceSwitch(
  owner: SessionOwnerDialogState,
  reenterDeepLink: () => void
): void {
  // A session without a project lives in User: focusing it binds the User desktop.
  focusDestination(ownerDataScope(owner.workspaceId));
  reenterDeepLink();
}
