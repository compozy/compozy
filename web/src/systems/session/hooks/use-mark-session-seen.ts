import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "@compozy/ui";

import { markSessionSeen } from "../adapters/session-mark-seen-api";
import { invalidateSessionMutationQueries } from "../lib/session-query-invalidation";
import type { SessionPayload } from "../types";
import { requireWorkspace } from "./session-action-workspace";
import { useAbortableMutationRequest } from "./use-abortable-mutation-request";
import type { SessionActionTarget } from "./use-session-actions";

const MARK_SEEN_FAILED = "Couldn't mark the session as seen.";

/** Clears a session's Needs you until it happens again and refreshes the catalogs that list it. */
export function useMarkSessionSeen() {
  const queryClient = useQueryClient();
  const request = useAbortableMutationRequest();
  return useMutation({
    mutationFn: ({ id, workspace_id, profile_name }: SessionActionTarget) =>
      request(signal =>
        markSessionSeen(requireWorkspace(workspace_id ?? ""), id, profile_name, signal)
      ),
    onSettled: (_data, _error, { id, workspace_id }) => {
      void invalidateSessionMutationQueries(queryClient, requireWorkspace(workspace_id ?? ""), id);
    },
  });
}

/** The Mark as seen verb a row menu or a session header offers; failures read as a toast. */
export function useMarkSessionSeenAction() {
  const markSeen = useMarkSessionSeen();
  return {
    pending: markSeen.isPending,
    onMarkSeen: (session: SessionPayload) => {
      if (markSeen.isPending) return;
      markSeen.mutate(session, {
        onError: error =>
          toast.error(error instanceof Error && error.message ? error.message : MARK_SEEN_FAILED),
      });
    },
  };
}
