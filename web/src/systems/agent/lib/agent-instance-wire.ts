import type { AgentInstancePayload, AgentPayload } from "../types";

/** An agent standing in for a deleted definition: it explains itself but can't be changed. */
export function isOrphanAgent(agent: Pick<AgentPayload, "definition_digest">) {
  return agent.definition_digest === "";
}

/** The definition is gone but its instances remain: enough of it to explain what happened. */
export function agentFromOrphanInstance(instance: AgentInstancePayload): AgentPayload {
  return {
    name: instance.agent_name,
    label: instance.label,
    kind: "stays_on",
    provider: instance.provider,
    prompt: "",
    origin: "global",
    definition_digest: "",
    resolved_appearance: instance.resolved_appearance,
  } as unknown as AgentPayload;
}
