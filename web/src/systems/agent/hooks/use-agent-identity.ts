import { useQueries, useQuery } from "@tanstack/react-query";

import { useCurrentProfileName } from "@/systems/profiles";
import { useSelectedWorkspaceId } from "@/systems/workspace";

import type { AgentIdentityScope } from "../lib/agent-identity-context";
import { GLOBAL_SCOPE } from "../lib/agent-instance";
import { agentsListOptions } from "../lib/query-options";
import { resolveAgentIdentity } from "../lib/agent-identity";

/**
 * Every caller of one scope shares a single cached catalog read, however many rows it has. Scope
 * comes from the stores rather than from the workspace and profile queries, so a leaf in every row
 * stays one observer.
 */
export function useAgentIdentityCatalog({
  workspace,
  profile,
  enabled = true,
}: AgentIdentityScope) {
  const selectedWorkspace = useSelectedWorkspaceId();
  const currentProfile = useCurrentProfileName();
  const target = workspace === undefined ? selectedWorkspace : workspace;
  const layer = target === GLOBAL_SCOPE ? null : target;
  const query = useQuery({
    ...agentsListOptions(layer || null, profile || currentProfile),
    enabled,
  });
  return enabled ? query.data : undefined;
}

type AgentOwner = { workspace_id?: string | null; profile_name: string };

function ownerKey(owner: AgentOwner) {
  return JSON.stringify([owner.workspace_id || null, owner.profile_name]);
}

/** Aggregate lists share one catalog query per recorded owner. */
export function useAgentIdentityByOwner(owners: readonly AgentOwner[]) {
  const unique = [...new Map(owners.map(owner => [ownerKey(owner), owner])).values()];
  const reads = useQueries({
    queries: unique.map(owner => ({
      ...agentsListOptions(owner.workspace_id || null, owner.profile_name),
      enabled: Boolean(owner.profile_name),
    })),
  });
  const catalogs = new Map(unique.map((owner, index) => [ownerKey(owner), reads[index]?.data]));
  return (name: string, owner: AgentOwner) =>
    resolveAgentIdentity(name, catalogs.get(ownerKey(owner)));
}
