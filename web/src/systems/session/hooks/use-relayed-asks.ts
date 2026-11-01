import { useQuery } from "@tanstack/react-query";

import { useAgentInstances } from "@/systems/agent";

import { sessionRelayedInteractionsOptions } from "../lib/query-options";
import type { SessionPendingInteraction } from "../types";

/** Only a main session relays; a child's asks stay in its own dock. */
export function useRelayedAsks(sessionId: string, workspaceId: string) {
  const instances = useAgentInstances();
  const instance = instances.find(candidate => candidate.main_session_id === sessionId);
  const { data } = useQuery(
    sessionRelayedInteractionsOptions(workspaceId, sessionId, Boolean(instance))
  );
  const asks = (data ?? []).filter(
    (ask: SessionPendingInteraction) =>
      ask.kind === "permission" && ask.status === "pending" && ask.session_id !== sessionId
  );
  return { asks, instance, instances };
}
