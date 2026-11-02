import { useQuery } from "@tanstack/react-query";

import { useCurrentProfileName } from "@/systems/profiles";
import { useSelectedWorkspaceId } from "@/systems/workspace";

import type { AgentIdentityScope } from "../lib/agent-identity-context";
import { GLOBAL_SCOPE } from "../lib/agent-instance";
import { agentsListOptions } from "../lib/query-options";

/**
 * Every caller of one scope shares a single cached catalog read, however many rows it has. Scope
 * comes from the stores rather than from the workspace and profile queries, so a leaf in every row
 * stays one observer.
 */
export function useAgentIdentityCatalog({ workspace, profile }: AgentIdentityScope) {
  const selectedWorkspace = useSelectedWorkspaceId();
  const currentProfile = useCurrentProfileName();
  const target = workspace === undefined ? selectedWorkspace : workspace;
  const layer = target === GLOBAL_SCOPE ? null : target;
  return useQuery(agentsListOptions(layer || null, profile || currentProfile)).data;
}
