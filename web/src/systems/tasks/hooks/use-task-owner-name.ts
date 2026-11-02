import type { AgentIdentityScope } from "@/systems/agent";
import { useAgentIdentityView } from "@/systems/agent/hooks/use-agent-identity-view";

import { taskOwnerLabel } from "../lib/task-formatters";

export type TaskOwner = NonNullable<Parameters<typeof taskOwnerLabel>[0]>;

/** An agent-session ref may be a session id, so only a catalog match speaks as an agent. */
export function useTaskOwnerName(owner?: TaskOwner | null, agentScope?: AgentIdentityScope) {
  const identity = useAgentIdentityView(owner?.ref ?? "", agentScope);
  return owner?.kind === "agent_session" && identity.known ? identity.label : taskOwnerLabel(owner);
}
