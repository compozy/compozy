import { useAgentIdentityView } from "@/systems/agent/hooks/use-agent-identity-view";

import { taskOwnerLabel } from "../lib/task-formatters";

export type TaskOwner = NonNullable<Parameters<typeof taskOwnerLabel>[0]>;

/** An agent-session ref may be a session id, so only a catalog match speaks as an agent. */
export function useTaskOwnerName(owner?: TaskOwner | null) {
  const identity = useAgentIdentityView(owner?.ref ?? "");
  return owner?.kind === "agent_session" && identity.known ? identity.label : taskOwnerLabel(owner);
}
