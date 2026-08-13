import type { SessionOwnerDialogState } from "@/systems/session";
import { enableGlobalScope, setActiveWorkspaceId } from "@/systems/workspace";

export function confirmSessionWorkspaceSwitch(
  owner: SessionOwnerDialogState,
  options: { isGlobal: boolean },
  reenterDeepLink: () => void
): void {
  // The home row is never a selectable workspace — writing its id into the
  // store would poison the remembered project selection.
  if (options.isGlobal) {
    enableGlobalScope();
  } else {
    setActiveWorkspaceId(owner.workspaceId);
  }
  reenterDeepLink();
}
