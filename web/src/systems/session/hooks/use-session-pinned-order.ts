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
 * authorized in. A reorder patches every served Pinned page at once, restores the ones still
 * showing the patch when the daemon refuses, and rereads the workspace's lists either way. Absent
 * (`undefined`) without a pair: an aggregate list shows pins of several pairs and offers no reorder.
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
      // Each patched page with what it held and what the patch wrote, so a failure restores only
      // pages still showing the patch and keeps any newer read.
      const patched: Array<{
        key: readonly unknown[];
        before: CatalogEnvelope;
        written: CatalogEnvelope;
      }> = [];
      for (const [key, envelope] of previous) {
        if (!envelope || (key.at(-1) as { sort?: string } | undefined)?.sort !== "pinned") continue;
        const written: CatalogEnvelope = {
          ...envelope,
          pages: envelope.pages.map(page => ({
            ...page,
            sessions: applySessionSlotOrder(page.sessions, sessionIds),
          })),
        };
        // The cache may structurally share the patch, so compare against what it actually holds.
        const held = queryClient.setQueryData<CatalogEnvelope>(key, written);
        if (held) patched.push({ key, before: envelope, written: held });
      }
      return { patched };
    },
    onError: (error, _sessionIds, context) => {
      for (const { key, before, written } of context?.patched ?? []) {
        if (queryClient.getQueryData<CatalogEnvelope>(key) === written) {
          queryClient.setQueryData<CatalogEnvelope>(key, before);
        }
      }
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
