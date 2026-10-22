import { useSelector } from "@xstate/store-react";
import { isHydrated, rehydrateStore } from "@xstate/store/persist";
import { activeWorkspaceSelectors, activeWorkspaceStore } from "../stores/active-workspace-store";
import type { BreadthMode, Focus } from "../lib/destination";

export {
  activeWorkspaceStore,
  clearActiveWorkspaceSelection,
  disableAllProjects,
  enableAllProjects,
  focusDestination,
  focusUser,
  setActiveWorkspaceId,
  toggleAllProjects,
} from "../stores/active-workspace-store";

export function useSelectedWorkspaceId(): string | null {
  return useSelector(activeWorkspaceSelectors.selectedWorkspaceId);
}

/** Stored breadth; surfaces read the resolved value through `useActiveWorkspace`. */
export function useBreadthMode(): BreadthMode {
  return useSelector(activeWorkspaceSelectors.breadthMode);
}

/** Stored focus; surfaces read the resolved value through `useActiveWorkspace`. */
export function useFocus(): Focus {
  return useSelector(activeWorkspaceSelectors.focus);
}

export function isActiveWorkspaceStoreHydrated(): boolean {
  return isHydrated(activeWorkspaceStore);
}

export function rehydrateActiveWorkspaceStore(): Promise<void> {
  return rehydrateStore(activeWorkspaceStore);
}
