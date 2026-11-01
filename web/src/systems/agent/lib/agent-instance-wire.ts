import type { AgentInstancePayload, AgentPayload } from "../types";

/** The definition is gone but its instances remain: enough of it to explain what happened. */
export function agentFromOrphanInstance(instance: AgentInstancePayload): AgentPayload {
  return {
    name: instance.agent_name,
    label: instance.label,
    kind: "stays_on",
    provider: instance.provider ?? "",
    prompt: "",
    origin: "global",
    definition_digest: "",
    resolved_appearance: instance.resolved_appearance,
  } as unknown as AgentPayload;
}
