import { useAgentInstances } from "@/systems/agent";

import { useSessionLabel } from "../components/session-messages/use-session-label";

/**
 * The session a "Sent to" card names, and whether it is a stays-on agent that is paused right now
 * (S12): a `delivery: held` send stays held only while that is true.
 */
export function useSessionSentTarget({
  sessionId,
  workspaceId,
  currentWorkspaceId,
}: {
  sessionId: string;
  workspaceId: string;
  currentWorkspaceId: string;
}) {
  const target = useSessionLabel({ sessionId, workspaceId, currentWorkspaceId, agentName: null });
  const paused = useAgentInstances().some(
    instance =>
      sessionId !== "" && instance.main_session_id === sessionId && instance.state === "paused"
  );
  return { target, paused };
}
