/**
 * Call-instance counts for catalog rows.
 *
 * Roster trail pills answer "who is working / resting" from daemon call
 * records: `N running` is `state=running`. `parked` is a child-lifecycle fact
 * the wire does not project yet, so it stays zero rather than being guessed
 * from session stop reasons.
 */
import type { CallPayload } from "@/systems/agent-comms";

export interface AgentFleetCallInstances {
  running: number;
  parked: number;
}

export function emptyAgentFleetCallInstances(): AgentFleetCallInstances {
  return { running: 0, parked: 0 };
}

export function countAgentFleetCallInstances(
  calls: readonly Pick<CallPayload, "agent" | "state">[]
): Map<string, AgentFleetCallInstances> {
  const counts = new Map<string, AgentFleetCallInstances>();
  for (const call of calls) {
    const agent = call.agent?.trim();
    if (!agent || call.state !== "running") continue;
    const current = counts.get(agent) ?? emptyAgentFleetCallInstances();
    current.running += 1;
    counts.set(agent, current);
  }
  return counts;
}

export function formatAgentFleetCallInstanceLabel(
  instances: AgentFleetCallInstances
): string | null {
  const parts: string[] = [];
  if (instances.running > 0) parts.push(`${instances.running} running`);
  if (instances.parked > 0) parts.push(`${instances.parked} parked`);
  return parts.length > 0 ? parts.join(", ") : null;
}
