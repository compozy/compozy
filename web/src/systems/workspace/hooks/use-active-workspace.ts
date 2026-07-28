import { useSelectedWorkspaceId } from "./use-active-workspace-store";
import { useWorkspaces } from "./use-workspaces";
import { selectActiveWorkspace } from "../lib/active-workspace";
import {
  clearActiveWorkspaceSelection,
  setActiveWorkspaceId,
} from "../stores/active-workspace-store";

export function useActiveWorkspace() {
  const selectedWorkspaceId = useSelectedWorkspaceId();
  const query = useWorkspaces();

  const activeWorkspace = selectActiveWorkspace(query.data ?? [], selectedWorkspaceId);

  return {
    ...query,
    workspaces: query.data ?? [],
    hasWorkspaces: (query.data?.length ?? 0) > 0,
    hasHydrated: true,
    selectedWorkspaceId,
    activeWorkspace,
    activeWorkspaceId: activeWorkspace?.id ?? null,
    setActiveWorkspaceId,
    clearActiveWorkspaceSelection,
  };
}
