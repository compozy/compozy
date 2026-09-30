import { useInfiniteQuery } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";
import { sessionCatalogOptions, type SessionPayload } from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";
import { pickLastCreatedSession } from "../lib/last-created-session";

export interface SessionLaunchCatalog {
  sessions: SessionPayload[];
  ready: boolean;
  workspaceId: string | null;
  resolveLatest: () => Promise<SessionPayload | null>;
}

/** The newest unarchived workspace session, independent of activity ordering. */
export function useSessionLaunchCatalog(): SessionLaunchCatalog {
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = runtimeWorkspaceId?.trim() || null;
  const { params } = useProfileReadScope();
  const query = useInfiniteQuery({
    ...sessionCatalogOptions({
      workspace_id: workspaceId ?? undefined,
      limit: 1,
      sort: "created",
      archive: "exclude",
      ...params,
    }),
    enabled: workspaceId !== null,
  });
  return {
    sessions: query.data?.pages[0]?.sessions ?? [],
    ready: workspaceId !== null && !query.isLoading,
    workspaceId,
    resolveLatest: async () => {
      const result = await query.refetch({ throwOnError: true, cancelRefetch: false });
      return pickLastCreatedSession(result.data?.pages[0]?.sessions ?? []);
    },
  };
}
