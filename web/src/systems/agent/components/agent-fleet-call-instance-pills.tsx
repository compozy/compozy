import { Pill } from "@compozy/ui";

import type { AgentFleetCallInstances } from "../lib/agent-fleet-call-instances";

export function AgentFleetCallInstancePills({
  agentName,
  instances,
}: {
  agentName: string;
  instances: AgentFleetCallInstances | null;
}) {
  if (!instances) return null;
  return (
    <>
      {instances.running > 0 ? (
        <Pill data-testid={`agent-fleet-running-${agentName}`} size="xs" tone="neutral">
          <Pill.Dot size="sm" tone="success" />
          {instances.running} running
        </Pill>
      ) : null}
      {instances.parked > 0 ? (
        <Pill data-testid={`agent-fleet-parked-${agentName}`} size="xs" tone="neutral">
          <Pill.Dot size="sm" tone="neutral" />
          {instances.parked} parked
        </Pill>
      ) : null}
    </>
  );
}
