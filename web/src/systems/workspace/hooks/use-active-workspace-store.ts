import { useSelector } from "@xstate/store-react";
import { isHydrated, rehydrateStore } from "@xstate/store/persist";
import { activeWorkspaceSelectors, activeWorkspaceStore } from "../stores/active-workspace-store";

export {
  activeWorkspaceStore,
  clearActiveWorkspaceSelection,
  setActiveWorkspaceId,
} from "../stores/active-workspace-store";

export function useSelectedWorkspaceId(): string | null {
  return useSelector(activeWorkspaceSelectors.selectedWorkspaceId);
}

export function isActiveWorkspaceStoreHydrated(): boolean {
  return isHydrated(activeWorkspaceStore);
}

export function rehydrateActiveWorkspaceStore(): Promise<void> {
  return rehydrateStore(activeWorkspaceStore);
}
