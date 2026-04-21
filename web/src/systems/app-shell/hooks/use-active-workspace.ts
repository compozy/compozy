import { useEffect, useMemo } from "react";

import { useActiveWorkspaceStore } from "../stores/active-workspace-store";
import type { Workspace } from "../types";
import { useWorkspaces } from "./use-workspaces";

export type WorkspaceBootstrapStatus =
  | "loading"
  | "error"
  | "empty"
  | "single"
  | "many"
  | "resolved";

export interface UseActiveWorkspaceResult {
  status: WorkspaceBootstrapStatus;
  workspaces: Workspace[];
  activeWorkspace: Workspace | null;
  activeWorkspaceId: string | null;
  selectedWorkspaceId: string | null;
  isStaleSelection: boolean;
  isLoading: boolean;
  isError: boolean;
  error: Error | null;
  setActiveWorkspaceId: (workspaceId: string) => void;
  clearActiveWorkspaceSelection: () => void;
  refetch: () => void;
}

export function useActiveWorkspace(): UseActiveWorkspaceResult {
  const query = useWorkspaces();
  const selectedWorkspaceId = useActiveWorkspaceStore(state => state.selectedWorkspaceId);
  const setSelected = useActiveWorkspaceStore(state => state.setSelectedWorkspaceId);
  const clearSelected = useActiveWorkspaceStore(state => state.clearSelectedWorkspaceId);

  const workspaces = useMemo(() => query.data ?? [], [query.data]);
  const selectedWorkspace = useMemo(() => {
    if (!selectedWorkspaceId) {
      return null;
    }
    return workspaces.find(entry => entry.id === selectedWorkspaceId) ?? null;
  }, [workspaces, selectedWorkspaceId]);

  const isStaleSelection = Boolean(
    selectedWorkspaceId && query.isSuccess && workspaces.length > 0 && !selectedWorkspace
  );

  useEffect(() => {
    if (isStaleSelection) {
      clearSelected();
    }
  }, [clearSelected, isStaleSelection]);

  useEffect(() => {
    if (query.isSuccess && workspaces.length === 1 && !selectedWorkspaceId) {
      const only = workspaces[0];
      if (only) {
        setSelected(only.id);
      }
    }
  }, [query.isSuccess, setSelected, selectedWorkspaceId, workspaces]);

  const { status, activeWorkspace } = resolveBootstrap({
    queryStatus: query.status,
    workspaces,
    selectedWorkspace,
    selectedWorkspaceId,
  });

  return {
    status,
    workspaces,
    activeWorkspace,
    activeWorkspaceId: activeWorkspace?.id ?? null,
    selectedWorkspaceId,
    isStaleSelection,
    isLoading: query.isLoading,
    isError: query.isError,
    error: (query.error as Error | null) ?? null,
    setActiveWorkspaceId: setSelected,
    clearActiveWorkspaceSelection: clearSelected,
    refetch: () => {
      void query.refetch();
    },
  };
}

function resolveBootstrap(params: {
  queryStatus: "pending" | "error" | "success";
  workspaces: Workspace[];
  selectedWorkspace: Workspace | null;
  selectedWorkspaceId: string | null;
}): { status: WorkspaceBootstrapStatus; activeWorkspace: Workspace | null } {
  if (params.queryStatus === "pending") {
    return { status: "loading", activeWorkspace: null };
  }
  if (params.queryStatus === "error") {
    return { status: "error", activeWorkspace: null };
  }
  if (params.workspaces.length === 0) {
    return { status: "empty", activeWorkspace: null };
  }
  if (params.selectedWorkspace) {
    return { status: "resolved", activeWorkspace: params.selectedWorkspace };
  }
  if (params.workspaces.length === 1) {
    return { status: "single", activeWorkspace: params.workspaces[0] ?? null };
  }
  if (params.selectedWorkspaceId) {
    return { status: "many", activeWorkspace: null };
  }
  return { status: "many", activeWorkspace: null };
}
