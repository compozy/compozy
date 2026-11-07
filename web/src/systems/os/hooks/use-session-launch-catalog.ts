import { useInfiniteQuery } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";
import { sessionCatalogOptions, type SessionPayload } from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";
import { pickLastCreatedSession } from "../lib/last-created-session";

export interface SessionLaunchCatalog {
  sessions: SessionPayload[];
  ready: boolean;
  /** The scoped workspace, or null in Global (rows then carry their own workspace). */
  workspaceId: string | null;
  resolveLatest: () => Promise<SessionPayload | null>;
}

/**
 * The newest unarchived session for the dock's Sessions launcher, independent
 * of activity ordering. An automation run never counts as the newest session:
 * the launcher opens the operator's own work (#1355). Workspace scope reads that workspace; Global has no
 * runtime workspace, so it reads every workspace instead of never resolving.
 */
export function useSessionLaunchCatalog(): SessionLaunchCatalog {
  const { runtimeWorkspaceId, pending } = useActiveWorkspace();
  const workspaceId = runtimeWorkspaceId?.trim() || null;
  const { params } = useProfileReadScope();
  const query = useInfiniteQuery({
    ...sessionCatalogOptions({
      workspace_id: workspaceId ?? undefined,
      limit: 1,
      sort: "created",
      archive: "exclude",
      exclude_automation: true,
      ...params,
    }),
    enabled: !pending,
  });
  return {
    sessions: query.data?.pages[0]?.sessions ?? [],
    ready: !pending && !query.isLoading,
    workspaceId,
    resolveLatest: async () => {
      const result = await query.refetch({ throwOnError: true, cancelRefetch: false });
      return pickLastCreatedSession(result.data?.pages[0]?.sessions ?? []);
    },
  };
}
