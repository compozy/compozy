import { useEffect } from "react";

import {
  useActiveWorkspaceStore,
  useActiveWorkspaceStoreHasHydrated,
} from "./use-active-workspace-store";
import { useWorkspaces } from "./use-workspaces";
import { selectActiveWorkspace } from "../lib/active-workspace";

export function useActiveWorkspace() {
  const hasHydrated = useActiveWorkspaceStoreHasHydrated();
  const selectedWorkspaceId = useActiveWorkspaceStore(state => state.selectedWorkspaceId);
  const setSelectedWorkspaceId = useActiveWorkspaceStore(state => state.setSelectedWorkspaceId);
  const clearSelectedWorkspaceId = useActiveWorkspaceStore(state => state.clearSelectedWorkspaceId);
  const query = useWorkspaces();

  const activeWorkspace = hasHydrated
    ? selectActiveWorkspace(query.data ?? [], selectedWorkspaceId)
    : undefined;

  useEffect(() => {
    if (!hasHydrated) {
      return;
    }
    if (!query.data || query.data.length === 0) {
      return;
    }
    if (!selectedWorkspaceId) {
      setSelectedWorkspaceId(query.data[0].id);
      return;
    }
    if (!query.data.some(workspace => workspace.id === selectedWorkspaceId)) {
      clearSelectedWorkspaceId();
    }
  }, [
    clearSelectedWorkspaceId,
    hasHydrated,
    query.data,
    selectedWorkspaceId,
    setSelectedWorkspaceId,
  ]);

  return {
    ...query,
    workspaces: query.data ?? [],
    hasWorkspaces: (query.data?.length ?? 0) > 0,
    hasHydrated,
    selectedWorkspaceId,
    activeWorkspace,
    activeWorkspaceId: activeWorkspace?.id ?? null,
    setActiveWorkspaceId: setSelectedWorkspaceId,
    clearActiveWorkspaceSelection: clearSelectedWorkspaceId,
  };
}
