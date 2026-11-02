import type { AgentPayload } from "../types";

/** An extension-owned definition: read-only here, with its owner named. */
export function isAgentReadOnly(agent: Pick<AgentPayload, "package_owned">) {
  return agent.package_owned === true;
}

export function formatAgentOwner(_agent: Pick<AgentPayload, "package_owned">) {
  return "An extension";
}
