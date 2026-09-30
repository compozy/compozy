import { useSessionCatalog, type SessionPayload } from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";

export interface SessionLaunchCatalog {
  sessions: SessionPayload[];
  ready: boolean;
  workspaceId: string | null;
}

/** The newest unarchived workspace session, independent of activity ordering. */
export function useSessionLaunchCatalog(): SessionLaunchCatalog {
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = runtimeWorkspaceId?.trim() || null;
  const query = useSessionCatalog(
    workspaceId,
    { limit: 1, sort: "created", archive: "exclude" },
    workspaceId !== null,
    { facets: false }
  );
  return {
    sessions: query.sessions,
    ready: workspaceId !== null && !query.loading && !query.failed,
    workspaceId,
  };
}
