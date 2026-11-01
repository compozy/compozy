import { toast } from "@compozy/ui";

import { useCreateSession, type SessionPayload } from "@/systems/session";

/** A dead session continues as a recovery fork of itself. */
export function useForkDeadSession(
  session: SessionPayload,
  sessionId: string,
  workspaceId: string,
  onForked: (forked: SessionPayload) => void
) {
  const forkSession = useCreateSession();
  const fork = () => {
    forkSession.mutate(
      {
        agent_name: session.agent_name,
        parent_session_id: sessionId,
        lineage_kind: "recovery",
        workspace: workspaceId,
      },
      {
        onError: error => {
          toast.error(error instanceof Error ? error.message : "Failed to fork session.");
        },
        onSuccess: onForked,
      }
    );
  };
  return { fork, isForking: forkSession.isPending };
}
