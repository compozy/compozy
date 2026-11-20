import { useMutation, useQueryClient } from "@tanstack/react-query";

import { archiveSession } from "../adapters/session-archive-api";
import { sessionKeys } from "../lib/query-keys";
import { isSessionSidesQuery } from "../lib/session-side-query-options";

/**
 * "Close side chat" (US-008): stop if running, then archive — one archive request with `stop`.
 * The side chat runs under its session's profile.
 */
export function useCloseSideChat({
  workspaceId,
  parentSessionId,
  profile,
}: {
  workspaceId: string;
  parentSessionId: string;
  profile: string;
}) {
  const queryClient = useQueryClient();
  return useMutation<unknown, Error, { sideId: string }>({
    mutationFn: ({ sideId }) => archiveSession(workspaceId, sideId, profile, undefined, true),
    onSettled: (_data, _error, { sideId }) => {
      void queryClient.invalidateQueries({
        predicate: query => isSessionSidesQuery(query.queryKey, parentSessionId),
      });
      void queryClient.invalidateQueries({ queryKey: sessionKeys.detail(workspaceId, sideId) });
    },
  });
}
