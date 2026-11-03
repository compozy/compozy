import { useMutation, useQueryClient } from "@tanstack/react-query";

import { moveSessionToProfile } from "../adapters/session-profile-move-api";
import { invalidateSessionMutationQueries } from "../lib/session-query-invalidation";
import { requireWorkspace } from "./session-action-workspace";
import { useAbortableMutationRequest } from "./use-abortable-mutation-request";
import type { SessionActionTarget } from "./use-session-actions";

/** Moves the supplied row to another profile and refreshes the catalogs that listed it. */
export function useMoveSessionToProfile() {
  const queryClient = useQueryClient();
  const request = useAbortableMutationRequest();
  return useMutation({
    mutationFn: ({
      id,
      workspace_id,
      profile_name,
      targetProfile,
    }: SessionActionTarget & { targetProfile: string }) =>
      request(signal =>
        moveSessionToProfile(
          requireWorkspace(workspace_id ?? ""),
          id,
          profile_name,
          targetProfile,
          signal
        )
      ),
    onSettled: (_data, _error, { id, workspace_id }) => {
      void invalidateSessionMutationQueries(queryClient, requireWorkspace(workspace_id ?? ""), id);
    },
  });
}
