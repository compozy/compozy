import { useMutation, useQueryClient, type InfiniteData } from "@tanstack/react-query";
import { toast } from "@compozy/ui";

import { reorderPinnedSessions } from "../adapters/session-folders-api";
import { sessionKeys } from "../lib/query-keys";
import { applySessionSlotOrder } from "../lib/session-folders";
import type { SessionListPair } from "../lib/session-list-groups";
import type { SessionCatalogPageResponse } from "../types";

type CatalogEnvelope = InfiniteData<SessionCatalogPageResponse>;

/**
 * Manual order of the Pinned section for one (workspace, profile) pair, the scope its writes are
 * authorized in. A reorder patches every served Pinned page at once, restores them when the daemon
 * refuses, and rereads the workspace's lists either way. Absent (`undefined`) without a pair: an
 * aggregate list shows pins of several pairs and offers no reorder.
 */
export function useSessionPinnedOrder(
  pair: SessionListPair | null
): ((sessionIds: readonly string[]) => void) | undefined {
  const queryClient = useQueryClient();
  const workspaceId = pair?.workspaceId ?? "";
  const profile = pair?.profile ?? "";
  const catalogPrefix = [...sessionKeys.workspaceLists(workspaceId), "page"] as const;
  const mutation = useMutation({
    // Shares the layout writes' scope so rapid reorders reach the daemon in the order made.
    scope: { id: `session-layout:${workspaceId}:${profile.trim()}` },
    mutationFn: (sessionIds: readonly string[]) =>
      reorderPinnedSessions({ workspaceId, profile: profile || undefined }, sessionIds),
    onMutate: async (sessionIds: readonly string[]) => {
      await queryClient.cancelQueries({ queryKey: sessionKeys.workspaceLists(workspaceId) });
      const previous = queryClient.getQueriesData<CatalogEnvelope>({ queryKey: catalogPrefix });
      for (const [key, envelope] of previous) {
        if (!envelope || (key.at(-1) as { sort?: string } | undefined)?.sort !== "pinned") continue;
        queryClient.setQueryData<CatalogEnvelope>(key, {
          ...envelope,
          pages: envelope.pages.map(page => ({
            ...page,
            sessions: applySessionSlotOrder(page.sessions, sessionIds),
          })),
        });
      }
      return { previous };
    },
    onError: (error, _sessionIds, context) => {
      for (const [key, data] of context?.previous ?? []) queryClient.setQueryData(key, data);
      toast.error("Couldn't reorder the pinned sessions.", {
        description: error instanceof Error ? error.message : undefined,
      });
    },
    onSettled: () =>
      queryClient.invalidateQueries({ queryKey: sessionKeys.workspaceLists(workspaceId) }),
  });
  if (pair === null) return undefined;
  return sessionIds => mutation.mutate(sessionIds);
}
