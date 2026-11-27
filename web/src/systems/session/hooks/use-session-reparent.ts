import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "@compozy/ui";

import { isRemoteComputerId, type ComputerId } from "@/lib/computer-scope";
import { ownerDataScope, workspaceSelectorForScope } from "@/systems/workspace";

import { clearSessionParent, setSessionParent } from "../adapters/session-api";
import { sessionKeys } from "../lib/query-keys";
import { getSessionDisplayTitle } from "../lib/session-display-title";
import { invalidateWorkspaceSessionCatalog } from "../lib/session-query-invalidation";
import { sessionComputerOf } from "../lib/session-row-ref";
import type { SessionPayload } from "../types";

type ReparentVariables =
  | { kind: "adopt"; session: SessionPayload; parent: SessionPayload; computerId?: ComputerId }
  | { kind: "detach"; session: SessionPayload; computerId?: ComputerId };

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
 * session's own profile, so it routes under any profile lens. `blocked` is the
 * host's lineage batch lock: while a batch Make top-level runs, every single move
 * (row menu, drop, toast Undo) is declined so it cannot overlap that batch.
 */
export function useSessionReparent({
  blocked = () => false,
  computerOf,
}: {
  blocked?: () => boolean;
  computerOf?: (session: SessionPayload) => ComputerId | undefined;
} = {}) {
  const queryClient = useQueryClient();
  const mutation = useMutation({
    mutationFn: (variables: ReparentVariables) => {
      const remote = isRemoteComputerId(variables.computerId);
      if (variables.kind === "adopt") {
        return remote
          ? setSessionParent(
              workspacePath(variables.session),
              variables.session.id,
              variables.session.profile_name,
              variables.parent.id,
              undefined,
              variables.computerId
            )
          : setSessionParent(
              workspacePath(variables.session),
              variables.session.id,
              variables.session.profile_name,
              variables.parent.id
            );
      }
      return remote
        ? clearSessionParent(
            workspacePath(variables.session),
            variables.session.id,
            variables.session.profile_name,
            undefined,
            variables.computerId
          )
        : clearSessionParent(
            workspacePath(variables.session),
            variables.session.id,
            variables.session.profile_name
          );
    },
    onSuccess: (session, variables) => {
      queryClient.setQueryData(
        sessionKeys.forComputer(variables.computerId).detail(workspacePath(session), session.id),
        session
      );
    },
    onSettled: (_data, _error, variables) =>
      invalidateWorkspaceSessionCatalog(
        queryClient,
        workspacePath(variables.session),
        variables.computerId
      ),
  });
  const declined = () => mutation.isPending || blocked();
  const detach = (session: SessionPayload, computerId?: ComputerId) => {
    if (declined()) return;
    const resolvedComputer = computerId ?? computerOf?.(session);
    mutation.mutate(
      { kind: "detach", session, computerId: resolvedComputer },
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
  };
  return {
    pending: mutation.isPending,
    detachAsync: (session: SessionPayload, computerId?: ComputerId) =>
      mutation.mutateAsync({
        kind: "detach",
        session,
        computerId: computerId ?? computerOf?.(session),
      }),
    adoptAsync: (session: SessionPayload, parent: SessionPayload, computerId?: ComputerId) => {
      if (sessionComputerOf(session) !== sessionComputerOf(parent)) {
        return Promise.reject(new Error("Sessions stay on the computer that runs them."));
      }
      return mutation.mutateAsync({
        kind: "adopt",
        session,
        parent,
        computerId: computerId ?? computerOf?.(session),
      });
    },
    adopt: (session: SessionPayload, parent: SessionPayload, computerId?: ComputerId) => {
      if (declined()) return;
      if (sessionComputerOf(session) !== sessionComputerOf(parent)) return;
      const resolvedComputer = computerId ?? computerOf?.(session);
      mutation.mutate(
        { kind: "adopt", session, parent, computerId: resolvedComputer },
        {
          onError: failed,
          onSuccess: moved =>
            toast.success(
              `Moved “${getSessionDisplayTitle(session)}” under “${getSessionDisplayTitle(parent)}”`,
              { action: { label: "Undo", onClick: () => detach(moved, resolvedComputer) } }
            ),
        }
      );
    },
    detach,
  };
}
