import { useQuery } from "@tanstack/react-query";

import { agentContextOptions, taskContextBundleOptions } from "../lib/query-options";

interface QueryHookOptions {
  enabled?: boolean;
}

export function useAgentContext(options: QueryHookOptions = {}) {
  return useQuery(agentContextOptions(options.enabled ?? true));
}

export function useTaskContextBundle(options: QueryHookOptions = {}) {
  return useQuery(taskContextBundleOptions(options.enabled ?? true));
}
