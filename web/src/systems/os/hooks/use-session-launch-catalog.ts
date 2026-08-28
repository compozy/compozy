import { useSessions, type SessionPayload } from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";

export interface SessionLaunchCatalog {
  sessions: SessionPayload[];
  ready: boolean;
  workspaceId: string | null;
}

/** Workspace catalog for dock launch: live rows only, no worktree or archive lens. */
export function useSessionLaunchCatalog(): SessionLaunchCatalog {
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = runtimeWorkspaceId?.trim() || null;
  const query = useSessions(workspaceId, {
    enabled: workspaceId !== null,
    loadAll: true,
    filters: { limit: 100 },
  });
  return {
    sessions: query.data ?? [],
    ready: workspaceId !== null && query.isFetched && !query.isError,
    workspaceId,
  };
}
