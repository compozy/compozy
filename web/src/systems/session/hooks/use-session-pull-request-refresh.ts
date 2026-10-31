import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "@compozy/ui";

import { refreshSessionPullRequest } from "../adapters/session-pull-request-api";
import { pullRequestCauseText } from "../lib/pull-request-signal";
import { sessionKeys } from "../lib/query-keys";
import type { SessionPullRequestRefreshResult } from "../lib/session-pull-request";
import { invalidateSessionMutationQueries } from "../lib/session-query-invalidation";
import type { SessionPayload } from "../types";

/**
 * Explicit PR refresh for one session (US-008). The fresh facts land in the
 * session's detail read at once, then every read of the session is invalidated.
 * A failed refresh keeps the cached facts and says why in a toast.
 */
export function useSessionPullRequestRefresh(
  session: Pick<SessionPayload, "id" | "workspace_id" | "profile_name">
) {
  const queryClient = useQueryClient();
  const workspaceId = session.workspace_id ?? "";
  const mutation = useMutation<SessionPullRequestRefreshResult, Error>({
    mutationFn: () => refreshSessionPullRequest(session.id, session.profile_name || undefined),
    onSuccess: result => {
      const merge = (current: SessionPayload | undefined) =>
        current && {
          ...current,
          ...(result.branch ? { branch: result.branch } : {}),
          pull_request: result.pull_request,
          pull_request_lookup: result.pull_request_lookup,
        };
      queryClient.setQueryData<SessionPayload>(sessionKeys.detail(workspaceId, session.id), merge);
      queryClient.setQueriesData<SessionPayload>(
        { queryKey: sessionKeys.byIdRoot(session.id) },
        merge
      );
      const lookup = result.pull_request_lookup;
      if (lookup?.state === "error") toast.error(pullRequestCauseText(lookup.cause));
    },
    onError: error => {
      toast.error(error.message || "Couldn't refresh the pull request.");
    },
    onSettled: () => invalidateSessionMutationQueries(queryClient, workspaceId, session.id),
  });
  return {
    refreshing: mutation.isPending,
    refresh: () => {
      if (!mutation.isPending) mutation.mutate();
    },
  };
}
