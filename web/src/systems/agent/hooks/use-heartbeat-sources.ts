import { useQuery } from "@tanstack/react-query";

import { agentHeartbeatSourcesOptions } from "../lib/query-options";

/**
 * The kinds and scripts an agent's check-ins can read, for titles and limits. Only read while
 * something needs it; `null` until it answers, so callers fall back to the declared names.
 */
export function useHeartbeatSourceCatalog(
  name: string,
  workspace: string | null,
  profile: string,
  enabled: boolean
) {
  const { data } = useQuery({
    ...agentHeartbeatSourcesOptions(name, workspace, profile),
    enabled: enabled && name !== "",
  });
  return data ?? null;
}
