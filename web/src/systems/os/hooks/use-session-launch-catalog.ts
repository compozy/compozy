import { useQuery } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";
import { sessionLaunchPreviewOptions, type SessionPayload } from "@/systems/session";
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
  const { data, isLoading, refetch } = useQuery({
    // The idle read is a preload hint; clicks fetch the latest row independently of catalog wakes.
    ...sessionLaunchPreviewOptions({
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
    sessions: data?.sessions ?? [],
    ready: !pending && !isLoading,
    workspaceId,
    resolveLatest: async () => {
      const result = await refetch({ throwOnError: true });
      return pickLastCreatedSession(result.data?.sessions ?? []);
    },
  };
}
