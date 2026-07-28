import { useQuery } from "@tanstack/react-query";

import { agentContextOptions } from "../lib/query-options";
import type { AgentContextIdentity } from "../types";

interface QueryHookOptions {
  enabled?: boolean;
}

export function useAgentContext(identity: AgentContextIdentity, options: QueryHookOptions = {}) {
  return useQuery(agentContextOptions(identity, options.enabled ?? true));
}

export function useTaskContextBundle(
  identity: AgentContextIdentity,
  options: QueryHookOptions = {}
) {
  return useQuery({
    ...agentContextOptions(identity, options.enabled ?? true),
    select: context => context.task.bundle ?? null,
  });
}
