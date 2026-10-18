import type { SessionOwnerDialogState } from "@/systems/session";
import { enableGlobalScope, setActiveWorkspaceId } from "@/systems/workspace";

export function confirmSessionWorkspaceSwitch(
  owner: SessionOwnerDialogState,
  options: { isGlobal: boolean },
  reenterDeepLink: () => void
): void {
  // Global history has no project id. Preserve the remembered project while
  // changing only the data scope.
  if (options.isGlobal) {
    enableGlobalScope();
  } else {
    setActiveWorkspaceId(owner.workspaceId);
  }
  reenterDeepLink();
}
