import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "@compozy/ui";

import { ownerDataScope, workspaceSelectorForScope } from "@/systems/workspace";

import { clearSessionParent, setSessionParent } from "../adapters/session-api";
import { sessionKeys } from "../lib/query-keys";
import { getSessionDisplayTitle } from "../lib/session-display-title";
import { invalidateWorkspaceSessionCatalog } from "../lib/session-query-invalidation";
import type { SessionPayload } from "../types";

type ReparentVariables =
  | { kind: "adopt"; session: SessionPayload; parent: SessionPayload }
  | { kind: "detach"; session: SessionPayload };

function failed(error: unknown) {
  toast.error("Couldn't move the session.", {
    description: error instanceof Error ? error.message : undefined,
  });
}

function workspacePath(session: SessionPayload): string {
  return workspaceSelectorForScope(ownerDataScope(session.workspace_id));
}

/**
 * Nests a top-level session under another (a drop onto its row) and returns a
 * nested child to the top level (Make top-level, and the drop's Undo). Take over
 * uses the same detach request: the daemon turns a stopped spawned worker into a
 * top-level user session. The
 * daemon owns the lineage change and validates it; each request names the
 * session's own profile, so it routes under any profile lens.
 */
export function useSessionReparent() {
  const queryClient = useQueryClient();
  const mutation = useMutation({
    mutationFn: (variables: ReparentVariables) =>
      variables.kind === "adopt"
        ? setSessionParent(
            workspacePath(variables.session),
            variables.session.id,
            variables.session.profile_name,
            variables.parent.id
          )
        : clearSessionParent(
            workspacePath(variables.session),
            variables.session.id,
            variables.session.profile_name
          ),
    onSuccess: session => {
      queryClient.setQueryData(sessionKeys.detail(workspacePath(session), session.id), session);
    },
    onSettled: (_data, _error, variables) =>
      invalidateWorkspaceSessionCatalog(queryClient, workspacePath(variables.session)),
  });
  const detach = (session: SessionPayload) =>
    mutation.mutate(
      { kind: "detach", session },
      {
        onError: failed,
        onSuccess: () =>
          toast.success(
            session.type === "spawned"
              ? `You took over “${getSessionDisplayTitle(session)}”`
              : `“${getSessionDisplayTitle(session)}” is top-level again`
          ),
      }
    );
  return {
    pending: mutation.isPending,
    adopt: (session: SessionPayload, parent: SessionPayload) => {
      if (mutation.isPending) return;
      mutation.mutate(
        { kind: "adopt", session, parent },
        {
          onError: failed,
          onSuccess: moved =>
            toast.success(
              `Moved “${getSessionDisplayTitle(session)}” under “${getSessionDisplayTitle(parent)}”`,
              { action: { label: "Undo", onClick: () => detach(moved) } }
            ),
        }
      );
    },
    detach: (session: SessionPayload) => {
      if (!mutation.isPending) detach(session);
    },
  };
}
