import { useProfileReadScope } from "@/systems/profiles";
import { GLOBAL_SCOPE, useAgentInstances, useStaysOnAgents } from "@/systems/agent";

import { instancesInScope } from "../lib/stays-on-scope";

/** The stays-on agents a scope resolves that don't already work there. */
export function useUnplacedAgents(place: string) {
  const { destination } = useProfileReadScope();
  const instances = useAgentInstances().filter(instance => instance.profile_name === destination);
  const catalog = useStaysOnAgents(place === GLOBAL_SCOPE ? "" : place, true);
  const placed = new Set(instancesInScope(instances, place).map(instance => instance.agent_name));
  return {
    agents: (catalog.data ?? []).filter(agent => !placed.has(agent.name)),
    pending: catalog.isPending,
  };
}
