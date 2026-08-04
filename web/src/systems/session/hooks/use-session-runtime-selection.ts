import { useMutation, useQueryClient } from "@tanstack/react-query";

import { setSessionRuntime, SessionApiError } from "../adapters/session-api";
import { sessionKeys } from "../lib/query-keys";
import { invalidateWorkspaceSessionCatalog } from "../lib/session-query-invalidation";
import type { SetSessionRuntimeRequest } from "../types";

export interface SetSessionRuntimeVariables {
  id: string;
  request: SetSessionRuntimeRequest;
  signal: AbortSignal;
}

/** Persists next-prompt runtime intent and reconciles the exact session cache. */
export function useSetSessionRuntime(workspaceId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, request, signal }: SetSessionRuntimeVariables) =>
      setSessionRuntime(workspaceId, id, request, signal),
    onSuccess: (session, variables) => {
      if (variables.signal.aborted) return;
      queryClient.setQueryData(sessionKeys.detail(workspaceId, session.id), session);
      void invalidateWorkspaceSessionCatalog(queryClient, workspaceId);
    },
    onError: (error, variables) => {
      if (variables.signal.aborted) return;
      if (error instanceof SessionApiError && error.status === 409) {
        void queryClient.invalidateQueries({
          queryKey: sessionKeys.detail(workspaceId, variables.id),
        });
      }
    },
  });
}
