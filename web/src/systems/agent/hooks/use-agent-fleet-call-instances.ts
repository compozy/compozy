import { useInfiniteQuery } from "@tanstack/react-query";

import { callsListOptions, useAgentCommsScope } from "@/systems/agent-comms";

import {
  countAgentFleetCallInstances,
  type AgentFleetCallInstances,
} from "../lib/agent-fleet-call-instances";

/**
 * Workspace-wide running-call counts, grouped by agent.
 *
 * One filtered list, not one probe per row. Pills stay absent until the
 * population is complete — a partial page would under-count.
 */
export function useAgentFleetCallInstances(
  live: boolean,
  enabled: boolean
): ReadonlyMap<string, AgentFleetCallInstances> | null {
  const scope = useAgentCommsScope();
  const query = useInfiniteQuery(callsListOptions(scope, { state: "running" }, live, enabled));
  if (!query.isSuccess || query.hasNextPage) return null;
  return countAgentFleetCallInstances(query.data.pages.flatMap(page => page.items));
}
