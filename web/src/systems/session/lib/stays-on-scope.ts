import { GLOBAL_SCOPE, type AgentInstancePayload } from "@/systems/agent";

/** One scope's instances: a workspace id, or `@user` for Global. */
export function instancesInScope(instances: readonly AgentInstancePayload[], scope: string) {
  const workspaceId = scope === GLOBAL_SCOPE ? "" : scope;
  return instances.filter(instance => instance.workspace_id === workspaceId);
}
