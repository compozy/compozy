import { useSelector } from "@xstate/store-react";
import { isHydrated, rehydrateStore } from "@xstate/store/persist";
import { activeWorkspaceSelectors, activeWorkspaceStore } from "../stores/active-workspace-store";
import type { WorkspaceScopeMode } from "../lib/workspace-scope-mode";

export {
  activeWorkspaceStore,
  clearActiveWorkspaceSelection,
  disableGlobalScope,
  enableGlobalScope,
  setActiveWorkspaceId,
  toggleGlobalScope,
} from "../stores/active-workspace-store";

export function useSelectedWorkspaceId(): string | null {
  return useSelector(activeWorkspaceSelectors.selectedWorkspaceId);
}

export function useWorkspaceScopeMode(): WorkspaceScopeMode {
  return useSelector(activeWorkspaceSelectors.scope);
}

export function isActiveWorkspaceStoreHydrated(): boolean {
  return isHydrated(activeWorkspaceStore);
}

export function rehydrateActiveWorkspaceStore(): Promise<void> {
  return rehydrateStore(activeWorkspaceStore);
}
